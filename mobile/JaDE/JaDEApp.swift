import SwiftUI
import Security

enum PairingKeychain {
    private static let query: [String: Any] = [kSecClass as String:kSecClassGenericPassword, kSecAttrService as String:"JaDE.Sync", kSecAttrAccount as String:"pairing"]
    static func read() -> Pairing? {
        var q = query; q[kSecReturnData as String] = true; q[kSecMatchLimit as String] = kSecMatchLimitOne
        var result: CFTypeRef?
        guard SecItemCopyMatching(q as CFDictionary, &result) == errSecSuccess, let data = result as? Data else { return nil }
        return try? JSONDecoder().decode(Pairing.self, from: data)
    }
    static func save(_ pairing: Pairing) throws {
        let data = try JSONEncoder().encode(pairing)
        let attributes: [String: Any] = [kSecValueData as String:data, kSecAttrAccessible as String:kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly]
        var status = SecItemUpdate(query as CFDictionary, attributes as CFDictionary)
        if status == errSecItemNotFound { status = SecItemAdd(query.merging(attributes) {_,new in new} as CFDictionary, nil) }
        guard status == errSecSuccess else { throw SyncFailure(message: "Could not store pairing securely (\(status))") }
    }
}

@main struct JaDEApp: App {
    @StateObject private var store = SyncStore()
    @Environment(\.scenePhase) private var scenePhase
    var body: some Scene {
        WindowGroup {
            TabView {
                LibraryView(store: store).tabItem { Label("Notes", systemImage:"note.text") }
                MacHome().tabItem { Label("Mac files", systemImage:"desktopcomputer") }
            }
                .tint(Color(red: 0.08, green: 0.40, blue: 0.29))
                .task {
                    #if DEBUG
                    if ProcessInfo.processInfo.environment["JADE_OFFLINE_UI_TEST"] == "1" { return }
                    #endif
                    if let pairing = PairingKeychain.read() { try? store.configure(pairing) }; await store.sync()
                }
                .onOpenURL { url in
                    guard url.scheme == "jade", url.host == "pair", let components = URLComponents(url: url, resolvingAgainstBaseURL: false),
                          let endpoint = components.queryItems?.first(where: {$0.name == "endpoint"})?.value,
                          let token = components.queryItems?.first(where: {$0.name == "token"})?.value else { return }
                    do { let pairing = Pairing(endpoint:endpoint,token:token); try store.configure(pairing); try PairingKeychain.save(pairing); Task { await store.sync() } }
                    catch { store.message = error.localizedDescription }
                }
                .onChange(of: scenePhase) { _, phase in
                    if phase == .active { Task { await store.sync() } }
                    else { _ = store.persist() }
                }
        }
    }
}

struct LibraryView: View {
    @ObservedObject var store: SyncStore
    @State private var showSettings = false
    @State private var showNew = false
    @State private var filename = ""
    @State private var error = ""
    @State private var search = ""
    private let timer = Timer.publish(every: 10, on: .main, in: .common).autoconnect()
    @Environment(\.scenePhase) private var scenePhase
    var body: some View {
        NavigationStack {
            List {
                Section {
                    VStack(alignment: .leading, spacing: 8) {
                        Label(store.isSyncing ? "Syncing…" : store.paired ? "Your personal workspace" : "Offline workspace", systemImage: store.isSyncing ? "arrow.triangle.2.circlepath" : "externaldrive")
                            .font(.headline)
                        Text(store.storageError ?? store.message).font(.footnote).foregroundStyle(store.storageError == nil ? .secondary : Color.red)
                        if let date = store.lastSync { Text("Last checked \(date.formatted(date:.omitted,time:.shortened))").font(.caption).foregroundStyle(.secondary) }
                        HStack {
                            Button("Sync now") { Task { await store.sync() } }.buttonStyle(.bordered).disabled(store.isSyncing || !store.paired)
                            if !store.paired { Button("Pair with Mac") { showSettings = true }.buttonStyle(.borderedProminent) }
                        }
                    }.padding(.vertical, 6)
                }
                Section("Notes") {
                    if store.documents.isEmpty { Text("Create a note here, or pair with your Mac to download your workspace.").foregroundStyle(.secondary) }
                    ForEach(store.documents.filter { search.isEmpty || $0.path.localizedCaseInsensitiveContains(search) || $0.content.localizedCaseInsensitiveContains(search) }) { doc in
                        NavigationLink(value: doc.path) {
                            VStack(alignment: .leading, spacing: 6) {
                                Text(doc.path).font(.body.weight(.medium))
                                Text(store.storageError == nil ? doc.status : "Local save failed").font(.caption).foregroundStyle(doc.conflict != nil ? Color.orange : .secondary)
                            }.padding(.vertical, 3)
                        }
                    }
                }
                Section { Text("Edits save on this iPhone first. Sync runs while JaDE is open. ‘Synced with Mac’ means your Mac acknowledged that revision.").font(.footnote).foregroundStyle(.secondary) }
            }
            .navigationTitle("JaDE")
            .searchable(text: $search, prompt: "Find a note")
            .navigationDestination(for: String.self) { path in EditorScreen(store: store, path: path) }
            .toolbar {
                ToolbarItem(placement: .primaryAction) { Button { filename = ""; showNew = true } label: { Image(systemName:"square.and.pencil") }.accessibilityLabel("New note") }
                ToolbarItem(placement: .automatic) { Button { showSettings = true } label: { Image(systemName:"gearshape") }.accessibilityLabel("Sync settings") }
            }
            .alert("New note", isPresented: $showNew) {
                TextField("notes.md", text: $filename)
                Button("Create") { do { try store.create(filename.hasSuffix(".md") || filename.hasSuffix(".txt") ? filename : filename + ".md") } catch { self.error = error.localizedDescription } }
                Button("Cancel", role: .cancel) {}
            } message: { Text("Use a .md or .txt filename. You can include folders, such as journal/today.md.") }
            .alert("Could not complete action", isPresented: Binding(get:{!error.isEmpty},set:{if !$0{error=""}})) { Button("OK") { error="" } } message: { Text(error) }
            .sheet(isPresented: $showSettings) { SettingsScreen(store:store) }
            .onReceive(timer) { _ in if scenePhase == .active { Task { await store.sync() } } }
        }
    }
}

struct EditorScreen: View {
    @ObservedObject var store: SyncStore
    let path: String
    @State private var error = ""
    var body: some View {
        VStack(spacing: 0) {
            if let doc = store.document(path) {
                HStack(alignment: .top) {
                    Image(systemName: doc.conflict != nil ? "exclamationmark.triangle" : doc.dirty ? "clock" : "checkmark.circle")
                    Text(store.storageError ?? doc.status).font(.footnote)
                    Spacer()
                    Button { Task { await store.sync() } } label: { Image(systemName:"arrow.triangle.2.circlepath") }.disabled(store.isSyncing).accessibilityLabel("Sync now")
                }.padding().background(Color.green.opacity(0.07))
                if let remote = doc.conflict {
                    VStack(alignment: .leading, spacing: 8) {
                        Text("This note also changed on another device.").font(.headline)
                        DisclosureGroup("View incoming version") { ScrollView { Text(remote.content).font(.system(.footnote,design:.monospaced)).textSelection(.enabled).frame(maxWidth:.infinity,alignment:.leading) }.frame(maxHeight:180) }
                        Button("Keep both versions") { do { try store.keepBoth(path) } catch { self.error=error.localizedDescription } }.buttonStyle(.bordered)
                        Text("Your text becomes a separate conflict copy. The incoming version stays at this filename.").font(.caption).foregroundStyle(.secondary)
                    }.padding().background(Color.orange.opacity(0.10))
                }
                TextEditor(text: Binding(get:{store.document(path)?.content ?? ""}, set:{store.edit(path,content:$0)}))
                    .font(.system(.body, design: .monospaced)).padding(8)
                    .accessibilityLabel("Note text")
                    .autocorrectionDisabled()
            }
        }
        .navigationTitle((path as NSString).lastPathComponent)
        #if os(iOS)
        .navigationBarTitleDisplayMode(.inline)
        #endif
        .toolbar { if let doc=store.document(path) { ShareLink(item:doc.content) { Image(systemName:"square.and.arrow.up") }.accessibilityLabel("Export note text") } }
        .alert("Could not save both",isPresented:Binding(get:{!error.isEmpty},set:{if !$0{error=""}})) { Button("OK") {error=""} } message:{Text(error)}
    }
}

struct SettingsScreen: View {
    @ObservedObject var store: SyncStore
    @Environment(\.dismiss) private var dismiss
    @State private var endpoint = ""
    @State private var token = ""
    @State private var error = ""
    var body: some View {
        NavigationStack {
            Form {
                Section("Pair with your Mac") {
                    Text("Scan the pairing QR code shown on your Mac using the iPhone Camera, or enter its details below.").font(.footnote)
                    TextField("https://your-worker.workers.dev", text:$endpoint).autocorrectionDisabled()
                    SecureField("Pairing key", text:$token)
                    Button("Save and sync") {
                        do { let p=Pairing(endpoint:endpoint.trimmingCharacters(in:.whitespacesAndNewlines),token:token.trimmingCharacters(in:.whitespacesAndNewlines));try store.configure(p);try PairingKeychain.save(p);dismiss();Task{await store.sync()} }
                        catch {self.error=error.localizedDescription}
                    }
                    if !error.isEmpty {Text(error).foregroundStyle(.red)}
                }
                Section("This first version") {
                    Text("One Mac and one iPhone. Markdown and text files up to 512 KB each. Files are never automatically deleted. Rename and attachment sync are not included yet.")
                    Text("The pairing key grants access to this workspace. It is stored in your iPhone Keychain. The server can read your notes; end-to-end encryption is not enabled.")
                    Text("Keep JaDE open to finish syncing. Pending edits remain on your phone when offline or when your Mac is asleep.")
                }.font(.footnote)
            }.navigationTitle("Sync settings")
                .toolbar { Button("Done") { dismiss() } }
                .onAppear { endpoint=store.endpoint }
        }
    }
}
