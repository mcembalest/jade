import Foundation
import Combine

struct CloudProject: Codable, Identifiable {
    var id: String; var name: String; var enabled: Int; var lastSeen: String; var status: String
}
struct CloudMeta: Codable, Identifiable {
    var path: String; var revision: String; var writer: String; var updatedAt: String
    var bytes: Int; var macRevision: String; var macIssue: String
    var id: String { path }
}
struct CloudVersion: Codable {
    var path: String; var content: String; var revision: String
    var writer: String?; var updatedAt: String?; var macRevision: String?; var macIssue: String?
}
struct CloudHistoryItem: Codable, Identifiable {
    var revision: String; var writer: String; var updatedAt: String; var bytes: Int
    var id: String { revision }
}
struct CloudUpload: Codable {
    var path: String; var content: String; var baseRevision: String; var mutationId = UUID().uuidString
}
struct CloudDocument: Codable, Identifiable {
    var project: String; var path: String; var content: String; var baseContent: String; var revision: String
    var pending: CloudUpload?
    var cloudRevision = ""; var macRevision = ""; var macIssue = ""
    var conflict: CloudVersion?
    var id: String { project + ":" + path }
    var dirty: Bool { content != baseContent || revision.isEmpty }
    var status: String {
        if conflict != nil { return "Conflict · your draft is kept" }
        if let pending {
            return content == pending.content ? "Queued on iPhone · not yet stored in Cloudflare" : "Newer phone draft · earlier edit queued for Cloudflare"
        }
        if dirty { return "Draft saved on iPhone · not submitted" }
        if cloudRevision != revision { return "Newer cloud version available · fetch to review" }
        if !macIssue.isEmpty { return "Stored in Cloudflare · Mac needs attention" }
        return macRevision == revision ? "Stored in Cloudflare · applied on Mac" : "Stored in Cloudflare · Mac pending"
    }
}
private struct CloudCache: Codable {
    var projects: [CloudProject] = []
    var manifests: [String:[CloudMeta]] = [:]
    var documents: [String:CloudDocument] = [:]
}

@MainActor final class CloudStore: ObservableObject {
    @Published private var cache = CloudCache()
    @Published var message = "Enable a cloud project in JaDE Mac Connection."
    @Published var busy = false
    @Published var storageError: String?
    @Published var downloadProgress = ""
    @Published var backupMessage = "Backup status has not been checked."
    private var stopDownload = false
    private let disk: URL
    private let session: URLSession
    private var pairing: Pairing?
    private var unreadable = false
    var projects: [CloudProject] { cache.projects }
    var documents: [CloudDocument] { cache.documents.values.sorted { $0.id < $1.id } }
    init(directory: URL? = nil,session: URLSession = .shared) {
        self.session = session
        var folder = directory ?? FileManager.default.urls(for:.applicationSupportDirectory,in:.userDomainMask)[0].appendingPathComponent("JaDE")
        #if DEBUG
        if directory == nil, let id = ProcessInfo.processInfo.environment["JADE_UI_TEST_ID"], UUID(uuidString:id) != nil { folder = folder.appendingPathComponent("UITests/"+id) }
        #endif
        disk = folder.appendingPathComponent("cloud-projects.json")
        do {
            try FileManager.default.createDirectory(at:folder,withIntermediateDirectories:true)
            if FileManager.default.fileExists(atPath:disk.path) {
                cache = try JSONDecoder().decode(CloudCache.self,from:Data(contentsOf:disk))
                guard cache.documents.allSatisfy({ $0.key == $0.value.id && Self.validPath($0.value.path) }) else { throw SyncFailure(message:"Invalid saved project data") }
            }
        } catch { unreadable = true; storageError = "Cannot read saved cloud drafts. Original storage is preserved." }
    }
    static func validPath(_ path: String) -> Bool {
        let excluded: Set<String> = ["node_modules","vendor","build","dist","__pycache__","DerivedData"]
        return !path.isEmpty && path.utf8.count <= 768 && !path.contains("\\") && !path.unicodeScalars.contains(where: { $0.value < 32 || $0.value == 127 }) && path.split(separator:"/",omittingEmptySubsequences:false).allSatisfy { !$0.isEmpty && !$0.hasPrefix(".") && !excluded.contains(String($0)) }
    }
    func configure(_ pair: Pairing,allowLocalhost: Bool = false) throws {
        guard let url=URL(string:pair.endpoint), url.host != nil, url.user == nil, url.query == nil, url.fragment == nil,
              url.scheme == "https" || (allowLocalhost && url.scheme == "http" && url.host == "127.0.0.1") else { throw SyncFailure(message:"Use the paired HTTPS service") }
        guard pairing == nil || pairing?.endpoint == pair.endpoint else { throw SyncFailure(message:"Export project drafts before changing services") }
        pairing = pair
    }
    private func persist() throws {
        guard !unreadable else { throw SyncFailure(message:storageError ?? "Saved drafts need recovery") }
        do {
            let data = try JSONEncoder().encode(cache)
            #if os(iOS)
            try data.write(to:disk,options:[.atomic,.completeFileProtectionUntilFirstUserAuthentication])
            #else
            try data.write(to:disk,options:.atomic)
            #endif
            let handle=try FileHandle(forWritingTo:disk); defer { try? handle.close() }; try handle.synchronize(); storageError=nil
        } catch { storageError="Local draft save failed. Keep this app open and export your text."; throw error }
    }
    func manifest(_ project: String) -> [CloudMeta] { cache.manifests[project] ?? [] }
    func document(_ project: String,_ path: String) -> CloudDocument? { cache.documents[project+":"+path] }
    func offlineCount(_ project:String) -> Int { manifest(project).filter { document(project,$0.path) != nil }.count }
    func cancelDownload() { stopDownload = true }
    // A bounded, restartable fetch. Existing copies (including clean but older copies)
    // are untouched: replacing them remains the editor's explicit Fetch action.
    func downloadProject(_ project:String) async {
        guard !busy,!unreadable else { return };busy=true;stopDownload=false;defer { busy=false }
        var downloaded=0, failed=0
        do {
            let (_,r)=try await request(route(project,"files"));let files=r.files ?? []
            guard files.count<=2000,files.allSatisfy({Self.validPath($0.path) && $0.bytes>=0 && $0.bytes<=262144}) else { throw SyncFailure(message:"Invalid project listing") }
            cache.manifests[project]=files;try persist()
            for f in files {
                if stopDownload || Task.isCancelled { break }
                if document(project,f.path) != nil { continue }
                downloadProgress="Downloading \(f.path) · \(offlineCount(project))/\(files.count) saved"
                do {
                    let (_,reply)=try await request(route(project,"file",f.path))
                    guard let version=reply.file,version.path==f.path,version.content.utf8.count<=262144 else { throw SyncFailure(message:"Invalid cloud file") }
                    // The user may create a draft during the network await.
                    if document(project,f.path)==nil {
                        cache.documents[project+":"+f.path]=CloudDocument(project:project,path:f.path,content:version.content,baseContent:version.content,revision:version.revision,cloudRevision:version.revision,macRevision:version.macRevision ?? "",macIssue:version.macIssue ?? "")
                        try persist();downloaded += 1
                    }
                } catch {
                    failed += 1
                    if storageError != nil { throw error }
                    // Stop on connectivity loss instead of spending 20 seconds per file.
                    if error is URLError { break }
                }
            }
            downloadProgress="\(offlineCount(project))/\(files.count) files available offline. \(downloaded) downloaded; \(failed) failed."
            if stopDownload || Task.isCancelled { downloadProgress += " Download stopped." }
            message=downloadProgress+" Run Download again to resume. Existing phone copies were kept."
        } catch { downloadProgress="Download paused. Saved files are kept; run Download again to resume.";message=downloadProgress+" "+error.localizedDescription }
    }

    // Save the pre-resolution draft as a separate local document in the same atomic
    // cache write. Resolution only stages a draft; submission still uses server CAS.
    func resolve(_ project:String,_ path:String,content:String) throws {
        guard !busy,let doc=document(project,path),let remote=doc.conflict,doc.pending==nil else { throw SyncFailure(message:"No conflict ready to resolve") }
        guard content.utf8.count<=262144,!content.contains("\0") else { throw SyncFailure(message:"Text files up to 256 KB can be submitted") }
        let recovery="Recovered drafts/"+UUID().uuidString+"-"+String((path as NSString).lastPathComponent.suffix(80))
        let previous=cache
        cache.documents[project+":"+recovery]=CloudDocument(project:project,path:recovery,content:doc.content,baseContent:"",revision:"")
        cache.documents[project+":"+path]=CloudDocument(project:project,path:path,content:content,baseContent:remote.content,revision:remote.revision,cloudRevision:remote.revision,macRevision:remote.macRevision ?? "",macIssue:remote.macIssue ?? "")
        do { try persist() } catch { cache=previous;throw error }
        message="Resolution saved on iPhone. Original draft kept in \(recovery). Submit separately when ready."
    }
    func edit(_ project: String,_ path: String,content: String) {
        let key=project+":"+path
        guard !unreadable, cache.documents[key] != nil else { return }
        cache.documents[key]?.content=content
        do { try persist() } catch { message=error.localizedDescription }
    }
    func create(_ project: String,_ path: String) throws {
        guard Self.validPath(path), document(project,path)==nil, !manifest(project).contains(where:{$0.path==path}) else { throw SyncFailure(message:"Choose a new filename without hidden or parent folders") }
        cache.documents[project+":"+path]=CloudDocument(project:project,path:path,content:"",baseContent:"",revision:"")
        try persist()
    }
    private struct Response: Decodable {
        var projects:[CloudProject]?; var files:[CloudMeta]?; var file:CloudVersion?
        var acceptedRevision:String?; var error:String?; var revisions:[CloudHistoryItem]?
        var backup:BackupStatus?
    }
    private struct BackupStatus: Decodable { var lastSuccess:String;var error:String;var retentionDays:Int }
    private func request(_ path: String,body: CloudUpload? = nil) async throws -> (Int,Response) {
        guard let pairing, let url=URL(string:pairing.endpoint+path) else { throw SyncFailure(message:"Pair JaDE in Notes settings first") }
        var req=URLRequest(url:url);req.timeoutInterval=20
        req.setValue("Bearer "+pairing.token,forHTTPHeaderField:"Authorization")
        if let body { req.httpMethod="POST";req.setValue("application/json",forHTTPHeaderField:"Content-Type");req.httpBody=try JSONEncoder().encode(body) }
        let (data,r)=try await session.data(for:req)
        guard let r=r as? HTTPURLResponse else { throw SyncFailure(message:"No response from Cloudflare") }
        let decoded=try JSONDecoder().decode(Response.self,from:data)
        guard r.statusCode==200 || r.statusCode==409 else { throw SyncFailure(message:decoded.error ?? "Cloudflare unavailable") }
        return (r.statusCode,decoded)
    }
    private func route(_ project:String,_ action:String,_ path:String? = nil) -> String {
        var c=URLComponents();c.path="/v1/projects/"+project+"/"+action
        if let path { c.queryItems=[URLQueryItem(name:"path",value:path)] }
        return c.string!
    }
    private func retry(_ key: String) async throws {
        guard let doc=cache.documents[key], let pending=doc.pending else { return }
        let (status,r)=try await request(route(doc.project,"file"),body:pending)
        if status==409 {
            guard let conflict=r.file else { throw SyncFailure(message:r.error ?? "Conflict; export your draft before retrying") }
            cache.documents[key]?.conflict=conflict;cache.documents[key]?.pending=nil
        } else {
            guard r.acceptedRevision==pending.mutationId else { throw SyncFailure(message:"Cloudflare did not acknowledge the submission") }
            cache.documents[key]?.baseContent=pending.content;cache.documents[key]?.revision=pending.mutationId;cache.documents[key]?.pending=nil
            cache.documents[key]?.cloudRevision=r.file?.revision ?? pending.mutationId
            cache.documents[key]?.macRevision=r.file?.macRevision ?? "";cache.documents[key]?.macIssue=r.file?.macIssue ?? ""
        }
        try persist()
    }
    func refresh() async {
        guard !busy,!unreadable else { return };busy=true;defer { busy=false }
        do {
            try persist()
            // Only explicitly submitted snapshots are retried. Draft typing is never an upload.
            var waiting = 0
            for key in cache.documents.keys.sorted() {
                if cache.documents[key]?.pending != nil {
                    do { try await retry(key) } catch { waiting += 1 }
                }
            }
            let (_,r)=try await request("/v1/projects");cache.projects=r.projects ?? []
            for p in cache.projects {
                let (_,r)=try await request(route(p.id,"files"));let files=r.files ?? []
                guard files.allSatisfy({Self.validPath($0.path)}) else { throw SyncFailure(message:"Invalid project listing") }
                cache.manifests[p.id]=files
                for f in files {
                    let key=p.id+":"+f.path
                    cache.documents[key]?.cloudRevision=f.revision;cache.documents[key]?.macRevision=f.macRevision;cache.documents[key]?.macIssue=f.macIssue
                }
            }
            try persist();message = waiting > 0 ? "Cloudflare checked; \(waiting) queued submissions are still waiting. Phone drafts are retained." : "Cloudflare checked. Fetch and submit are separate; Mac delivery is shown per file."
            do {
                let (_,r)=try await request("/v1/backup")
                if let b=r.backup {
                    backupMessage=b.lastSuccess.isEmpty ? "First automatic backup is pending." : "Last backup: \(b.lastSuccess) · kept for \(b.retentionDays) days."
                    if !b.error.isEmpty { backupMessage += " "+b.error }
                }
            } catch { backupMessage="Backup status unavailable; last successful backup could not be checked." }
        } catch { message="Cloud connection paused: \(error.localizedDescription). Saved drafts and queued submissions are retained." }
    }
    func fetch(_ project: String,_ path: String,replaceDraft: Bool = false) async throws {
        guard !busy else { throw SyncFailure(message:"Another cloud operation is running") };busy=true;defer { busy=false }
        let key=project+":"+path
        if let doc=cache.documents[key],doc.pending != nil { throw SyncFailure(message:"Finish the queued submission before replacing this draft") }
        if let doc=cache.documents[key],doc.dirty && !replaceDraft { throw SyncFailure(message:"Your phone has a draft. Export it or confirm replacement before fetching.") }
        let (_,r)=try await request(route(project,"file",path));guard let f=r.file,Self.validPath(f.path),f.path==path,f.content.utf8.count<=262144 else { throw SyncFailure(message:"Invalid cloud file") }
        cache.documents[key]=CloudDocument(project:project,path:path,content:f.content,baseContent:f.content,revision:f.revision,cloudRevision:f.revision,macRevision:f.macRevision ?? "",macIssue:f.macIssue ?? "")
        try persist()
    }
    func submit(_ project: String,_ path: String) async {
        guard !busy else { return };busy=true;defer { busy=false }
        let key=project+":"+path
        do {
            guard let doc=cache.documents[key],doc.conflict==nil else { throw SyncFailure(message:"Resolve the cloud conflict or save a separate copy first") }
            guard doc.content.utf8.count<=262144,!doc.content.contains("\0") else { throw SyncFailure(message:"Text files up to 256 KB can be submitted") }
            if doc.pending==nil { cache.documents[key]?.pending=CloudUpload(path:path,content:doc.content,baseRevision:doc.revision);try persist() }
            try await retry(key)
            message="Submission checked; see this file's delivery status."
        } catch { message="Submission pending: \(error.localizedDescription). Your phone copy is retained." }
    }
    func saveCopy(_ project:String,_ path:String,as newPath:String) throws {
        guard let doc=document(project,path) else { return }
        try create(project,newPath);edit(project,newPath,content:doc.content)
        guard storageError==nil else { throw SyncFailure(message:storageError!) }
    }
    func history(_ project:String,_ path:String) async throws -> [CloudHistoryItem] {
        let (_,r)=try await request(route(project,"history",path));return r.revisions ?? []
    }
    func historicalContent(_ project:String,_ path:String,revision:String) async throws -> String {
        var c=URLComponents(string:route(project,"revision",path))!;c.queryItems?.append(URLQueryItem(name:"revision",value:revision))
        let (_,r)=try await request(c.string!);guard let f=r.file else { throw SyncFailure(message:"Revision unavailable") };return f.content
    }
}
