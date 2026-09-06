import SwiftUI

struct MacRoot: Codable, Identifiable { var id: String; var name: String }
struct MacEntry: Codable, Identifiable { var name: String; var directory: Bool; var id: String { name } }
struct MacReply: Codable {
    var roots: [MacRoot]?
    var entries: [MacEntry]?
    var content: String?
    var revision: String?
    var error: String?
    var saved: Bool?
    var truncated: Bool?
}
struct MacEnvelope: Codable { var result: MacReply?; var pending: Bool?; var expired: Bool?; var error: String? }
struct MacRequest: Codable {
    var id = UUID().uuidString
    var action: String
    var root = ""
    var path = ""
    var content: String?
    var revision: String?
}
struct MacDraft: Codable { var content: String; var revision: String }

@MainActor final class MacFiles: ObservableObject {
    @Published var roots: [MacRoot] = []
    @Published var message = "Connect to your awake Mac to browse enabled folders."
    @Published var busy = false
    @Published private(set) var drafts: [String: MacDraft] = [:]
    private var damaged = false
    private let disk: URL
    init() {
        var folder = FileManager.default.urls(for:.applicationSupportDirectory,in:.userDomainMask)[0].appendingPathComponent("JaDE")
        #if DEBUG
        if let id = ProcessInfo.processInfo.environment["JADE_UI_TEST_ID"], UUID(uuidString:id) != nil { folder = folder.appendingPathComponent("UITests/" + id) }
        #endif
        disk = folder.appendingPathComponent("mac-drafts.json")
        do {
            try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
            if FileManager.default.fileExists(atPath:disk.path) { drafts = try JSONDecoder().decode([String:MacDraft].self,from:Data(contentsOf:disk)) }
        } catch { damaged = true; message = "Cannot read saved Mac drafts; original storage has been preserved." }
    }
    func key(_ root: String,_ path: String) -> String { root + ":" + path }
    func draft(_ root: String,_ path: String) -> MacDraft? { drafts[key(root,path)] }
    func saveDraft(_ root: String,_ path: String,_ draft: MacDraft) throws {
        guard !damaged else { throw SyncFailure(message:"Draft storage needs recovery before editing.") }
        var next = drafts; next[key(root,path)] = draft
        try JSONEncoder().encode(next).write(to:disk,options:[.atomic,.completeFileProtectionUntilFirstUserAuthentication])
        let file = try FileHandle(forWritingTo:disk); try file.synchronize(); try file.close()
        drafts = next
    }
    private func http(_ path: String, body: MacRequest? = nil) async throws -> Data {
        guard let pair = PairingKeychain.read(), let url = URL(string:pair.endpoint + path), url.scheme == "https" else { throw SyncFailure(message:"Pair JaDE with your Mac in Notes settings first.") }
        var r = URLRequest(url:url); r.timeoutInterval = 15
        r.setValue("Bearer " + pair.token,forHTTPHeaderField:"Authorization")
        if let body { r.httpMethod = "POST"; r.setValue("application/json",forHTTPHeaderField:"Content-Type"); r.httpBody = try JSONEncoder().encode(body) }
        let (data,response) = try await URLSession.shared.data(for:r)
        guard let response = response as? HTTPURLResponse, response.statusCode == 200 else { throw SyncFailure(message:"Could not reach your Mac connection. Your draft stays on this phone.") }
        return data
    }
    func call(_ request: MacRequest) async throws -> MacReply {
        _ = try await http("/v1/remote/request",body:request)
        for _ in 0..<25 {
            try await Task.sleep(for:.seconds(1))
            let reply = try JSONDecoder().decode(MacEnvelope.self,from:await http("/v1/remote/result?id=" + request.id))
            if let result = reply.result {
                if let error = result.error { throw SyncFailure(message:error) }
                return result
            }
            if reply.expired == true { break }
        }
        throw SyncFailure(message:"Mac hasn't replied yet. Keep it awake and online. A submitted save may still arrive; your draft is retained. Reconnect before retrying.")
    }
    func connect() async {
        guard !busy else { return }; busy = true; defer { busy = false }
        do { roots = try await call(MacRequest(action:"roots")).roots ?? []; message = roots.isEmpty ? "Connected. Choose folders using ‘Choose JaDE Mac folders’ on your Mac." : "Connected to your Mac" }
        catch { message = error.localizedDescription }
    }
}

struct MacHome: View {
    @StateObject private var mac = MacFiles()
    var body: some View {
        NavigationStack {
            List {
                Section {
                    Text(mac.message).font(.callout)
                    Button(mac.busy ? "Connecting…" : "Connect / refresh") { Task { await mac.connect() } }.disabled(mac.busy)
                }
                Section("Folders on your Mac") {
                    ForEach(mac.roots) { root in NavigationLink(root.name) { MacFolder(mac:mac,root:root,path:"") } }
                }
                if !mac.drafts.isEmpty {
                    Section("Saved phone drafts") {
                        ForEach(mac.drafts.keys.sorted(),id: \.self) { key in
                            let parts = key.split(separator:":",maxSplits:1).map(String.init)
                            if parts.count == 2 {
                                NavigationLink(parts[1]) { MacEditor(mac:mac,root:MacRoot(id:parts[0],name:"Saved draft"),path:parts[1]) }
                            }
                        }
                    }
                }
                Section { Text("Edits stay as drafts on this phone until you tap Save to Mac. Your Mac must be awake and online. Folder permissions are managed on the Mac.").font(.footnote).foregroundStyle(.secondary) }
            }.navigationTitle("Mac files")
            .task { await mac.connect() }
        }
    }
}
struct MacFolder: View {
    @ObservedObject var mac: MacFiles
    let root: MacRoot
    let path: String
    @State private var entries: [MacEntry] = []
    @State private var message = ""
    @State private var busy = false
    @State private var create = false
    @State private var name = ""
    @State private var newPath: String?
    func child(_ name: String) -> String { path.isEmpty ? name : path + "/" + name }
    func refresh() async {
        busy = true; defer { busy = false }
        do { let r = try await mac.call(MacRequest(action:"list",root:root.id,path:path)); entries = r.entries ?? []; message = r.truncated == true ? "Showing the first 1,000 entries." : "" }
        catch { message = error.localizedDescription }
    }
    var body: some View {
        List {
            if busy { ProgressView("Reading Mac folder…") }
            if !message.isEmpty { Text(message).font(.footnote) }
            ForEach(entries.sorted { $0.directory != $1.directory ? $0.directory : $0.name < $1.name }) { item in
                if item.directory { NavigationLink { MacFolder(mac:mac,root:root,path:child(item.name)) } label: { Label(item.name,systemImage:"folder") } }
                else { NavigationLink { MacEditor(mac:mac,root:root,path:child(item.name)) } label: { Label(item.name,systemImage:"doc.text") } }
            }
        }.navigationTitle(path.isEmpty ? root.name : (path as NSString).lastPathComponent)
            .toolbar {
                Button { Task { await refresh() } } label: { Image(systemName:"arrow.clockwise") }.disabled(busy)
                Button { name = ""; create = true } label: { Image(systemName:"square.and.pencil") }.accessibilityLabel("New Mac file")
            }
            .alert("New file",isPresented:$create) {
                TextField("Filename with extension",text:$name).autocorrectionDisabled().textInputAutocapitalization(.never)
                Button("Create draft") { if !name.isEmpty && !name.contains("/") && !name.hasPrefix(".") { newPath = child(name) } }
                Button("Cancel",role:.cancel) {}
            } message: { Text("The file will be created when you tap Save to Mac.") }
            .navigationDestination(item:$newPath) { p in MacEditor(mac:mac,root:root,path:p,isNew:true) }
            .task { await refresh() }
    }
}
struct MacEditor: View {
    @ObservedObject var mac: MacFiles
    let root: MacRoot
    let path: String
    var isNew = false
    @State private var text = ""
    @State private var revision = ""
    @State private var status = "Loading…"
    @State private var loaded = false
    @State private var busy = false
    @State private var reload = false
    @State private var copyName = ""
    @State private var showCopy = false
    func load() async {
        busy = true; defer { busy = false }
        do {
            let r = try await mac.call(MacRequest(action:"read",root:root.id,path:path))
            text = r.content ?? ""; revision = r.revision ?? ""; loaded = true
            try mac.saveDraft(root.id,path,MacDraft(content:text,revision:revision)); status = "Loaded from Mac"
        } catch { status = error.localizedDescription }
    }
    func save(to target: String? = nil) async {
        busy = true; defer { busy = false }
        let snapshot = text; let dest = target ?? path
        do {
            try mac.saveDraft(root.id,path,MacDraft(content:snapshot,revision:revision))
            let r = try await mac.call(MacRequest(action:"write",root:root.id,path:dest,content:snapshot,revision:target == nil ? revision : "new"))
            if target == nil {
                revision = r.revision ?? revision
                try mac.saveDraft(root.id,path,MacDraft(content:text,revision:revision))
            }
            status = target == nil ? "Saved on Mac" : "Copy saved on Mac: \(dest)"
        } catch { status = error.localizedDescription }
    }
    var body: some View {
        VStack(spacing:0) {
            HStack { if busy { ProgressView() }; Text(status).font(.footnote); Spacer() }.padding().background(Color.green.opacity(0.08))
            TextEditor(text:Binding(get:{text},set:{ value in
                text = value
                do { try mac.saveDraft(root.id,path,MacDraft(content:value,revision:revision)); status = "Draft saved on iPhone · not sent to Mac" }
                catch { status = "Local save failed: " + error.localizedDescription }
            })).font(.system(.body,design:.monospaced)).autocorrectionDisabled().textInputAutocapitalization(.never).disabled(!loaded || busy).accessibilityLabel("Mac file text")
            Button(busy ? "Waiting for Mac…" : "Save to Mac") { Task { await save() } }.buttonStyle(.borderedProminent).padding().disabled(!loaded || busy)
        }.navigationTitle((path as NSString).lastPathComponent).navigationBarTitleDisplayMode(.inline)
            .toolbar { Menu {
                Button("Reload from Mac") { reload = true }
                Button("Save as a new file") { copyName = ""; showCopy = true }.disabled(!loaded || busy)
                ShareLink(item:text) { Text("Export phone draft") }
            } label: { Image(systemName:"ellipsis.circle") } }
            .confirmationDialog("Replace this phone draft with the current Mac file? Export your draft first if you need to keep it.",isPresented:$reload,titleVisibility:.visible) { Button("Reload from Mac",role:.destructive) { Task { await load() } } }
            .alert("Save a copy",isPresented:$showCopy) {
                TextField("New filename",text:$copyName).autocorrectionDisabled().textInputAutocapitalization(.never)
                Button("Save copy") {
                    if !copyName.isEmpty && !copyName.contains("/") && !copyName.hasPrefix(".") {
                        let parent = (path as NSString).deletingLastPathComponent
                        let target = parent.isEmpty ? copyName : parent + "/" + copyName
                        Task { await save(to:target) }
                    }
                }
                Button("Cancel",role:.cancel) {}
            }
            .task {
                if let draft = mac.draft(root.id,path) { text = draft.content; revision = draft.revision; loaded = true; status = "Recovered phone draft · reload to check Mac version" }
                else if isNew { revision = "new"; loaded = true; status = "New phone draft · not on Mac yet" }
                else { await load() }
            }
    }
}
