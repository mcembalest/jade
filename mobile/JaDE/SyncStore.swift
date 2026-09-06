import Foundation
import Combine

struct RemoteFile: Codable, Equatable {
    var path: String
    var content: String
    var revision: String
    var acks: [String: String]
}
struct Mutation: Codable, Equatable {
    var path: String
    var content: String
    var baseRevision: String
    var mutationId: String = UUID().uuidString
    var deviceId: String = "iphone"
}
struct Document: Codable, Identifiable, Equatable {
    var id: String { path }
    var path: String
    var content: String = ""
    var baseContent: String = ""
    var baseRevision: String = ""
    var pending: Mutation?
    var conflict: RemoteFile?
    var acks: [String: String] = [:]
    var dirty: Bool { content != baseContent || baseRevision.isEmpty || pending != nil }
    var status: String {
        if conflict != nil { return "Conflict · both versions kept" }
        if dirty { return "Saved on iPhone · pending sync" }
        if acks["mac"] == baseRevision { return "Synced with Mac" }
        return "Uploaded · Mac pending"
    }
}
struct Pairing: Codable { var endpoint: String; var token: String }
struct SyncFailure: LocalizedError {
    var message: String
    var errorDescription: String? { message }
}

@MainActor final class SyncStore: ObservableObject {
    @Published var documents: [Document] = []
    @Published var message = "Your notes are saved on this iPhone."
    @Published var storageError: String?
    @Published var isSyncing = false
    @Published var lastSync: Date?
    @Published var paired = false
    private(set) var endpoint = ""
    private var token = ""
    private let fileURL: URL
    private let session: URLSession
    private var unreadable = false
    private var syncingAgain = false

    init(directory: URL? = nil, session: URLSession = .shared) {
        self.session = session
        var directory = directory ?? FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask)[0].appendingPathComponent("JaDE", isDirectory: true)
        #if DEBUG
        if let id = ProcessInfo.processInfo.environment["JADE_UI_TEST_ID"], UUID(uuidString:id) != nil {
            directory = directory.appendingPathComponent("UITests/" + id, isDirectory:true)
        }
        #endif
        fileURL = directory.appendingPathComponent("vault.json")
        do {
            try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
            if FileManager.default.fileExists(atPath: fileURL.path) {
                documents = try JSONDecoder().decode([Document].self, from: Data(contentsOf: fileURL))
                guard Set(documents.map(\.path)).count == documents.count, documents.allSatisfy({ Self.validPath($0.path) }) else { throw SyncFailure(message: "Invalid saved workspace") }
            }
        } catch {
            unreadable = true
            storageError = "Cannot read local notes. The saved file has been left untouched: \(error.localizedDescription)"
        }
    }
    static func validPath(_ path: String) -> Bool {
        guard path.utf8.count <= 240, ["md", "txt"].contains((path as NSString).pathExtension.lowercased()) else { return false }
        return path.split(separator: "/", omittingEmptySubsequences: false).allSatisfy { part in
            !part.isEmpty && !part.hasPrefix(".") && part.trimmingCharacters(in: .whitespaces) == String(part) &&
            part.range(of: "^[a-zA-Z0-9 _().-]+$", options: .regularExpression) != nil
        }
    }
    func configure(_ pairing: Pairing, allowLocalhost: Bool = false) throws {
        guard let url = URL(string: pairing.endpoint), url.host != nil,
              (url.scheme == "https" || (url.scheme == "http" && allowLocalhost && url.host == "127.0.0.1")),
              url.user == nil, url.query == nil, url.fragment == nil, pairing.token.count >= 32 else {
            throw SyncFailure(message: "Use the HTTPS server address and pairing key from your Mac.")
        }
        let address = pairing.endpoint.trimmingCharacters(in: CharacterSet(charactersIn: "/"))
        guard endpoint.isEmpty || endpoint == address else { throw SyncFailure(message: "Export your notes before switching to a different workspace.") }
        endpoint = address; token = pairing.token; paired = true
    }
    @discardableResult func persist() -> Bool {
        guard !unreadable else { return false }
        do {
            let data = try JSONEncoder().encode(documents)
            #if os(iOS)
            try data.write(to: fileURL, options: [.atomic, .completeFileProtectionUntilFirstUserAuthentication])
            #else
            try data.write(to: fileURL, options: .atomic)
            #endif
            let handle = try FileHandle(forWritingTo: fileURL)
            defer { try? handle.close() }
            try handle.synchronize()
            storageError = nil
            return true
        } catch {
            storageError = "Not saved on iPhone: \(error.localizedDescription). Keep JaDE open and export your text."
            return false
        }
    }
    func create(_ path: String, content: String = "") throws {
        guard !unreadable, Self.validPath(path), !documents.contains(where: {$0.path == path}) else { throw SyncFailure(message: "Choose a unique .md or .txt filename using letters, numbers, spaces, dashes or underscores.") }
        documents.append(Document(path: path, content: content)); documents.sort { $0.path < $1.path }
        if !persist() { throw SyncFailure(message: storageError ?? "Could not save") }
    }
    func edit(_ path: String, content: String) {
        guard !unreadable, let i = index(path) else { return }
        documents[i].content = content
        _ = persist() // Local durability is independent of network availability.
    }
    func document(_ path: String) -> Document? { documents.first { $0.path == path } }
    private func index(_ path: String) -> Int? { documents.firstIndex { $0.path == path } }
    func keepBoth(_ path: String) throws {
        guard let i = index(path), let remote = documents[i].conflict else { return }
        let ext = (path as NSString).pathExtension
        let stem = (path as NSString).deletingPathExtension
        let copy = "\(stem) (iPhone conflict \(UUID().uuidString.prefix(8))).\(ext)"
        guard Self.validPath(copy) else { throw SyncFailure(message: "Filename is too long to make a conflict copy. Export your text first.") }
        documents.append(Document(path: copy, content: documents[i].content))
        documents[i] = Document(path: path, content: remote.content, baseContent: remote.content, baseRevision: remote.revision, acks: remote.acks)
        guard persist() else { throw SyncFailure(message: storageError ?? "Could not save") }
    }
    private func request<T: Decodable>(_ method: String, _ path: String, body: Data? = nil, as: T.Type) async throws -> (Int, T) {
        guard let url = URL(string: endpoint + path) else { throw SyncFailure(message: "Pair with your Mac first") }
        var req = URLRequest(url: url); req.httpMethod = method; req.httpBody = body; req.timeoutInterval = 20
        req.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        req.setValue("application/json", forHTTPHeaderField: "Content-Type")
        let (data, response) = try await session.data(for: req)
        guard let response = response as? HTTPURLResponse else { throw SyncFailure(message: "Invalid server response") }
        guard response.statusCode == 200 || response.statusCode == 409 else {
            let detail = (try? JSONDecoder().decode(ServerError.self, from: data).error) ?? "Server unavailable"
            throw SyncFailure(message: "\(detail) (\(response.statusCode))")
        }
        return (response.statusCode, try JSONDecoder().decode(T.self, from: data))
    }
    private struct Snapshot: Decodable { var files: [RemoteFile] }
    private struct ServerError: Decodable { var error: String }
    private struct Result: Decodable { var acceptedRevision: String?; var file: RemoteFile? }
    private struct Ack: Encodable { var path: String; var revision: String; var deviceId = "iphone" }
    private func upload(_ path: String) async throws -> RemoteFile? {
        guard let i = index(path), let mutation = documents[i].pending else { return nil }
        let (code, result) = try await request("POST", "/v1/files", body: JSONEncoder().encode(mutation), as: Result.self)
        guard let j = index(path) else { return nil }
        if code == 409 {
            guard let remote = result.file else { throw SyncFailure(message: "Conflict response has no recoverable document") }
            documents[j].conflict = remote; documents[j].pending = nil
        } else {
            guard result.acceptedRevision == mutation.mutationId else { throw SyncFailure(message: "Server did not acknowledge this edit") }
            documents[j].baseContent = mutation.content
            documents[j].baseRevision = mutation.mutationId
            documents[j].pending = nil
            if let remote = result.file { documents[j].acks = remote.acks }
        }
        guard persist() else { throw SyncFailure(message: storageError ?? "Could not save sync receipt") }
        return result.file
    }
    func sync() async {
        guard paired else { message = "Pair with your Mac to start syncing. Offline editing is available."; return }
        guard !isSyncing else { syncingAgain = true; return }
        guard persist() else { return }
        isSyncing = true; message = "Syncing…"
        defer { isSyncing = false }
        do {
            let (_, snapshot) = try await request("GET", "/v1/files", as: Snapshot.self)
            var remotes: [String: RemoteFile] = [:]
            for remote in snapshot.files {
                guard Self.validPath(remote.path), remote.content.utf8.count <= 512 * 1024, remotes[remote.path] == nil else { throw SyncFailure(message: "Server returned an invalid document") }
                remotes[remote.path] = remote
            }
            for path in Set(documents.map(\.path)).union(remotes.keys).sorted() {
                if index(path) == nil, let remote = remotes[path] {
                    documents.append(Document(path: path, content: remote.content, baseContent: remote.content, baseRevision: remote.revision, acks: remote.acks))
                    guard persist() else { throw SyncFailure(message: storageError ?? "Could not save downloaded note") }
                }
                // Retry the identical persisted operation before reconciliation.
                if let i = index(path), documents[i].pending != nil, let latest = try await upload(path) { remotes[path] = latest }
                guard var i = index(path) else { continue }
                if let remote = remotes[path] {
                    documents[i].acks = remote.acks
                    if documents[i].content == remote.content {
                        documents[i].baseContent = remote.content; documents[i].baseRevision = remote.revision; documents[i].conflict = nil
                    } else if documents[i].baseRevision != remote.revision {
                        if !documents[i].dirty {
                            documents[i].content = remote.content; documents[i].baseContent = remote.content; documents[i].baseRevision = remote.revision; documents[i].conflict = nil
                        } else { documents[i].conflict = remote }
                    }
                }
                guard persist() else { throw SyncFailure(message: storageError ?? "Could not save") }
                if documents[i].conflict != nil { continue }
                if documents[i].dirty {
                    guard documents[i].content.utf8.count <= 512 * 1024 else { throw SyncFailure(message: "\(path) is saved locally but exceeds the 512 KB sync limit") }
                    documents[i].pending = Mutation(path: path, content: documents[i].content, baseRevision: documents[i].baseRevision)
                    guard persist() else { throw SyncFailure(message: storageError ?? "Could not stage upload") }
                    _ = try await upload(path)
                }
                guard let j = index(path) else { continue }; i = j
                if documents[i].conflict != nil { continue }
                let revision = documents[i].baseRevision
                if documents[i].acks["iphone"] != revision {
                    let (_, receipt) = try await request("POST", "/v1/ack", body: JSONEncoder().encode(Ack(path: path, revision: revision)), as: Result.self)
                    if let k = index(path), let remote = receipt.file { documents[k].acks = remote.acks }
                }
                guard persist() else { throw SyncFailure(message: storageError ?? "Could not save receipt") }
            }
            documents.sort { $0.path < $1.path }; _ = persist()
            lastSync = Date()
            message = documents.contains { $0.conflict != nil } ? "A conflict needs attention. Both versions are kept." : "Connected · check each note for Mac delivery."
        } catch {
            message = "Sync paused: \(error.localizedDescription) Your local edits are kept."
        }
        if syncingAgain { syncingAgain = false } // The foreground timer retries; never spin on a failed connection.
    }
}
