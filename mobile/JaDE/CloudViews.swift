import SwiftUI

struct CloudProjectsView: View {
    @StateObject private var store=CloudStore()
    var body: some View {
        List {
            Section {
                Text(store.storageError ?? store.message).font(.footnote)
                Button("Refresh cloud projects") { Task { await store.refresh() } }.disabled(store.busy)
                if store.busy { ProgressView("Checking Cloudflare…") }
            }
            Section("Available through Cloudflare") {
                ForEach(store.projects) { project in
                    NavigationLink { CloudProjectView(store:store,project:project) } label: {
                        VStack(alignment:.leading,spacing:5) {
                            Text(project.name)
                            Text(project.enabled==1 ? "Available with Mac off" : "Paused · cloud copy retained").font(.caption).foregroundStyle(.secondary)
                        }
                    }
                }
                if store.projects.isEmpty { Text("On the Mac, use JaDE → Cloud projects to opt in a specific folder. Enabling Mac files alone doesn't upload it.").font(.footnote) }
            }
            Section {
                Text("Cloudflare holds the last uploaded files. Typing saves a phone draft. Submit to Cloudflare queues delivery to your Mac, even while it is off. Fetching a newer version remains a separate action.").font(.footnote).foregroundStyle(.secondary)
                Text(store.backupMessage).font(.footnote).foregroundStyle(.secondary)
            }
        }.navigationTitle("Cloud projects")
            .task {
                do {
                    #if DEBUG
                    if ProcessInfo.processInfo.environment["JADE_CLOUD_LOCAL_UI_TEST"]=="1" {
                        try store.configure(Pairing(endpoint:"http://127.0.0.1:8799",token:"local-test-key-not-for-production-123456"),allowLocalhost:true)
                    } else if let pair=PairingKeychain.read() { try store.configure(pair) }
                    #else
                    if let pair=PairingKeychain.read() { try store.configure(pair) }
                    #endif
                    await store.refresh()
                }
                catch { store.message=error.localizedDescription }
            }
    }
}
struct CloudProjectView: View {
    @ObservedObject var store:CloudStore
    let project:CloudProject
    @State private var search=""
    @State private var new=false
    @State private var name=""
    @State private var error=""
    @State private var destination:String?
    var paths:[String] { Array(Set(store.manifest(project.id).map(\.path)+store.documents.filter{$0.project==project.id}.map(\.path))).sorted().filter{search.isEmpty || $0.localizedCaseInsensitiveContains(search)} }
    var body: some View {
        List {
            Section {
                Text("Last Mac check: \(project.lastSeen)").font(.caption).foregroundStyle(.secondary)
                Text(project.status).font(.footnote)
                Text(store.storageError ?? store.message).font(.footnote)
            }
            Section("Offline access") {
                Text("\(store.offlineCount(project.id))/\(store.manifest(project.id).count) files saved on this iPhone").font(.footnote)
                Button("Download missing files for offline use") { Task { await store.downloadProject(project.id) } }.disabled(store.busy)
                if store.busy { Button("Stop download") { store.cancelDownload() } }
                if !store.downloadProgress.isEmpty { Text(store.downloadProgress).font(.caption) }
                Text("Resumes after interruption. Existing phone copies and drafts stay unchanged; fetch updates individually.").font(.caption).foregroundStyle(.secondary)
            }
            Section("Files and phone drafts") {
                ForEach(paths,id:\.self) { path in
                    NavigationLink { CloudEditor(store:store,project:project,path:path) } label: {
                        VStack(alignment:.leading,spacing:5) {
                            Text(path)
                            if let doc=store.document(project.id,path) { Text(doc.status).font(.caption).foregroundStyle(.secondary) }
                            else if let meta=store.manifest(project.id).first(where:{$0.path==path}) {
                                Text(!meta.macIssue.isEmpty ? "Mac needs attention" : meta.macRevision==meta.revision ? "Cloudflare + Mac" : "Cloudflare · Mac pending").font(.caption).foregroundStyle(.secondary)
                            }
                        }
                    }
                }
            }
        }.navigationTitle(project.name).searchable(text:$search,prompt:"Find a project file")
            .toolbar {
                Button { Task { await store.refresh() } } label: { Image(systemName:"arrow.clockwise") }.disabled(store.busy)
                Button { name="";new=true } label: { Image(systemName:"square.and.pencil") }.accessibilityLabel("New cloud project file")
            }
            .alert("New phone draft",isPresented:$new) {
                TextField("folder/filename.py",text:$name).autocorrectionDisabled().textInputAutocapitalization(.never)
                Button("Create") { do { try store.create(project.id,name);destination=name } catch { self.error=error.localizedDescription } }
                Button("Cancel",role:.cancel) {}
            } message: { Text("Nothing is uploaded until you submit the draft.") }
            .alert("Could not create",isPresented:Binding(get:{!error.isEmpty},set:{if !$0{error=""}})) { Button("OK") { error="" } } message: { Text(error) }
            .navigationDestination(item:$destination) { CloudEditor(store:store,project:project,path:$0) }
    }
}
struct CloudEditor: View {
    @ObservedObject var store:CloudStore
    let project:CloudProject
    let path:String
    @State private var error=""
    @State private var replace=false
    @State private var copy=false
    @State private var copyPath=""
    @State private var history=false
    @State private var resolving=false
    var body: some View {
        VStack(spacing:0) {
            if let doc=store.document(project.id,path) {
                VStack(alignment:.leading,spacing:6) {
                    Text(store.storageError ?? doc.status).font(.footnote.weight(.medium))
                    if !doc.macIssue.isEmpty { Text(doc.macIssue).font(.caption).foregroundStyle(.orange) }
                    if doc.pending != nil { Text("This submitted snapshot will retry when you refresh Cloud projects. Newer typing stays a separate draft.").font(.caption) }
                    if let conflict=doc.conflict {
                        DisclosureGroup("View conflicting cloud version") {
                            ScrollView { Text(conflict.content).font(.system(.footnote,design:.monospaced)).textSelection(.enabled) }.frame(maxHeight:140)
                        }
                        Button("Resolve conflict…") { resolving=true }
                        Text("Compare the original, phone and cloud versions. Your original draft is saved separately when resolving.").font(.caption)
                    }
                }.frame(maxWidth:.infinity,alignment:.leading).padding().background(Color.green.opacity(0.08))
                TextEditor(text:Binding(get:{store.document(project.id,path)?.content ?? ""},set:{store.edit(project.id,path,content:$0)}))
                    .font(.system(.body,design:.monospaced)).autocorrectionDisabled().textInputAutocapitalization(.never)
                    .disabled(store.busy).accessibilityLabel("Cloud project text")
                Text(store.message).font(.caption).foregroundStyle(.secondary).padding(.horizontal)
                HStack {
                    Button("Fetch cloud version") { replace=true }.buttonStyle(.bordered)
                    Button("Submit to Cloudflare") { Task { await store.submit(project.id,path) } }.buttonStyle(.borderedProminent)
                }.padding().disabled(store.busy)
            } else {
                if store.busy { ProgressView("Fetching cloud copy…") }
                Text(error.isEmpty ? "Fetch this file from Cloudflare. Your Mac can be off." : error).padding()
                Button("Fetch cloud version") { Task { await fetch() } }.buttonStyle(.bordered).disabled(store.busy)
            }
        }.navigationTitle((path as NSString).lastPathComponent).navigationBarTitleDisplayMode(.inline)
            .toolbar { Menu {
                Button("Refresh delivery status") { Task { await store.refresh() } }
                Button("Revision history") { history=true }
                Button("Save draft as a separate file") { copyPath="";copy=true }
                if let doc=store.document(project.id,path) { ShareLink(item:doc.content) { Text("Export phone draft") } }
            } label: { Image(systemName:"ellipsis.circle") } }
            .confirmationDialog("Fetch the current cloud version? This replaces the phone draft. Export it or save a separate copy first if needed.",isPresented:$replace,titleVisibility:.visible) {
                Button("Fetch and replace phone draft",role:.destructive) { Task { await fetch(replaceDraft:true) } }
            }
            .alert("Save a separate draft",isPresented:$copy) {
                TextField("New project path",text:$copyPath).autocorrectionDisabled().textInputAutocapitalization(.never)
                Button("Save copy") {
                    do { try store.saveCopy(project.id,path,as:copyPath);store.message="Copy saved on phone. Open it in the project list to submit." }
                    catch { self.error=error.localizedDescription }
                }
                Button("Cancel",role:.cancel) {}
            }
            .alert("Cloud action paused",isPresented:Binding(get:{!error.isEmpty && store.document(project.id,path) != nil},set:{if !$0{error=""}})) { Button("OK") { error="" } } message: { Text(error) }
            .sheet(isPresented:$history) { CloudHistoryView(store:store,project:project,path:path) }
            .sheet(isPresented:$resolving) { CloudConflictView(store:store,project:project,path:path) }
            .task { if store.document(project.id,path)==nil { await fetch() } }
    }
    func fetch(replaceDraft:Bool=false) async {
        do { try await store.fetch(project.id,path,replaceDraft:replaceDraft);error="" }
        catch { self.error=error.localizedDescription }
    }
}
struct CloudConflictView: View {
    @ObservedObject var store:CloudStore
    let project:CloudProject
    let path:String
    @Environment(\.dismiss) var dismiss
    @State private var merged=""
    @State private var error=""
    var body: some View {
        NavigationStack {
            List {
                if let doc=store.document(project.id,path),let cloud=doc.conflict {
                    Section("Versions to compare") {
                        DisclosureGroup("Common starting version") { Text(doc.baseContent).font(.system(.footnote,design:.monospaced)).textSelection(.enabled) }
                        DisclosureGroup("Your phone draft") { Text(doc.content).font(.system(.footnote,design:.monospaced)).textSelection(.enabled) }
                        DisclosureGroup("Cloud version") { Text(cloud.content).font(.system(.footnote,design:.monospaced)).textSelection(.enabled) }
                    }
                    Section("Choose the result") {
                        Button("Use phone draft") { merged=doc.content }
                        Button("Use cloud version") { merged=cloud.content }
                        TextEditor(text:$merged).frame(minHeight:220).font(.system(.body,design:.monospaced)).autocorrectionDisabled().textInputAutocapitalization(.never).accessibilityLabel("Conflict resolution text")
                        Text("You can combine the text manually. Saving keeps the original phone draft as a separate file and stages this result. Nothing is submitted yet.").font(.caption)
                        Button("Save resolution and preserve original") {
                            do { try store.resolve(project.id,path,content:merged);dismiss() }
                            catch { self.error=error.localizedDescription }
                        }.disabled(store.busy)
                    }
                }
                if !error.isEmpty { Text(error).foregroundStyle(.orange) }
            }.navigationTitle("Resolve conflict").toolbar { Button("Cancel") { dismiss() } }
                .onAppear { merged=store.document(project.id,path)?.content ?? "" }
        }
    }
}
struct CloudHistoryView: View {
    @ObservedObject var store:CloudStore
    let project:CloudProject
    let path:String
    @Environment(\.dismiss) var dismiss
    @State private var items:[CloudHistoryItem]=[]
    @State private var preview=""
    @State private var selected=false
    @State private var error=""
    var body: some View {
        NavigationStack {
            List {
                if !error.isEmpty { Text(error).foregroundStyle(.orange) }
                if selected {
                    Section("Selected revision") {
                        Text(preview).font(.system(.footnote,design:.monospaced)).textSelection(.enabled)
                        ShareLink(item:preview) { Text("Export revision") }
                    }
                }
                Section("Last 50 accepted revisions") {
                    ForEach(items) { item in
                        Button("\(item.updatedAt) · \(item.writer)") {
                            Task {
                                do { preview=try await store.historicalContent(project.id,path,revision:item.revision);selected=true }
                                catch { self.error=error.localizedDescription }
                            }
                        }
                    }
                }
            }.navigationTitle("Revision history").toolbar { Button("Done") { dismiss() } }
                .task { do { items=try await store.history(project.id,path) } catch { self.error=error.localizedDescription } }
        }
    }
}
