import SwiftUI

@MainActor private func makeSanjanaStore() -> SanjanaStore {
    #if DEBUG
    if ProcessInfo.processInfo.environment["JADE_OFFLINE_UI_TEST"]=="1" { return SanjanaStore(pairing:{nil}) }
    #endif
    return SanjanaStore(pairing:PairingKeychain.read)
}

struct SanjanaSprite: View {
    @Environment(\.accessibilityReduceMotion) private var reduced
    @Environment(\.scenePhase) private var phase
    var still:Bool
    @State private var frame=0
    @State private var wave=true
    private static let frames:[[UIImage]] = {
        guard let sheet=UIImage(named:"Sanjana")?.cgImage else { return [] }
        return [0,3].map { row in (0..<(row==0 ? 6 : 4)).compactMap { column in
            sheet.cropping(to:CGRect(x:column*192,y:row*208,width:192,height:208)).map { UIImage(cgImage:$0) }
        } }
    }()
    var body: some View {
        Group {
            if !Self.frames.isEmpty {
                Image(uiImage:Self.frames[(still || reduced) ? 0 : (wave ? 1 : 0)][(still || reduced) ? 0 : frame]).resizable().interpolation(.none).scaledToFit()
            }
        }.frame(width:144,height:156).accessibilityLabel("Sanjana")
            .onTapGesture { frame=0;wave=true }
            .task(id:"\(still)-\(reduced)-\(phase)") {
                frame=0;guard !still,!reduced,phase == .active else { return }
                while !Task.isCancelled {
                    let delays=wave ? [140,140,140,280] : [280,110,110,140,140,320]
                    do { try await Task.sleep(for:.milliseconds(delays[frame])) } catch { return }
                    if frame+1>=delays.count { frame=0;wave=false } else { frame += 1 }
                }
            }
    }
}

struct SanjanaView: View {
    @StateObject private var store=makeSanjanaStore()
    @AppStorage("sanjana.still") private var still=false
    @AppStorage("sanjana.hidden") private var hidden=false
    @Environment(\.scenePhase) private var phase
    var body: some View {
        NavigationStack {
            ScrollView {
                VStack(alignment:.leading,spacing:20) {
                    VStack(spacing:6) {
                        if !hidden {
                            if store.state?.notebook?.avatar=="sanjana" {SanjanaSprite(still:still)}
                            else if let avatar=store.state?.notebook?.avatar,let encoded=avatar.split(separator:",").last,let data=Data(base64Encoded:String(encoded)),let image=UIImage(data:data) {Image(uiImage:image).resizable().scaledToFit().frame(width:144,height:156)}
                            else {Text(String((store.state?.notebook?.name ?? "Companion").prefix(2))).font(.largeTitle).frame(height:100)}
                        }
                        Text("A LITTLE COMPANY").font(.caption.weight(.semibold)).tracking(2)
                        Text(store.state?.notebook?.name.isEmpty == false ? store.state!.notebook!.name : "Your companion").font(.largeTitle.weight(.semibold))
                        Text("Research shaped by you.").font(.subheadline).foregroundStyle(.secondary).multilineTextAlignment(.center)
                    }.frame(maxWidth:.infinity).padding(.vertical)
                    Text(store.storageError ?? store.status).font(.footnote).foregroundStyle(.secondary).accessibilityIdentifier("Sanjana status")
                    if store.busy { ProgressView() }
                    Button(store.state?.paused == true ? "Resume research everywhere" : "Pause research everywhere") {
                        Task { await store.act(SanjanaAction(action:"settings",paused:store.state?.paused != true)) }
                    }.disabled(store.busy || store.state == nil)
                    if let setup=store.state?.providerStatus,!setup.isEmpty { Text(setup).font(.footnote).foregroundStyle(.secondary) }
                    DisclosureGroup("Discoveries · \(store.state?.pending?.count ?? 0) pending") {
                        VStack(alignment:.leading,spacing:16) {
                            ForEach(Array((store.state?.pending ?? []).enumerated()),id:\.offset) { _,message in row(message) }
                            if let error=store.state?.researchError,!error.isEmpty { Text(error).font(.footnote) }
                            Text("Shared findings stay here until the daily update.").font(.caption).foregroundStyle(.secondary)
                        }.padding(.top,8)
                    }
                    NavigationLink("Character, research & history") {CompanionNotebookView(store:store)}
                    Text("Daily updates").font(.title2.weight(.semibold))
                    ForEach(Array((store.state?.messages ?? []).filter { $0.proactive == true }.reversed().enumerated()),id:\.offset) { _,message in row(message) }
                    if !(store.state?.messages ?? []).contains(where:{$0.proactive == true}) { Text("Her updates will appear here as she finds things to share.").foregroundStyle(.secondary) }
                    Text("Daily research runs server-side when the cloud runtime is connected. Saved updates remain available offline.").font(.caption).foregroundStyle(.secondary)
                }.padding()
            }.background(Color(uiColor:.systemGroupedBackground)).navigationTitle(store.state?.notebook?.name.isEmpty == false ? store.state!.notebook!.name : "Companion").navigationBarTitleDisplayMode(.inline)
                .toolbar { Menu {
                    Button("Refresh updates") { Task { await store.refresh() } }.disabled(store.busy)
                    Toggle("Still animation",isOn:$still)
                    Toggle("Hide character on this phone",isOn:$hidden)
                } label: { Image(systemName:"ellipsis.circle") } }
                .task(id:phase) {
                    guard phase == .active else { return }
                    await store.refresh()
                    while !Task.isCancelled {
                        do { try await Task.sleep(for:.seconds(30)) } catch { return }
                        await store.refresh()
                    }
                }
        }
    }
    private func row(_ message:SanjanaMessage) -> some View {
        VStack(alignment:.leading,spacing:8) {
            Text(message.role=="user" ? "You" : store.state?.notebook?.name ?? "Companion").font(.caption.weight(.semibold)).foregroundStyle(.secondary)
            if let date=message.foundAt { Text(Date(timeIntervalSince1970:date/1000).formatted()).font(.caption).foregroundStyle(.secondary) }
            Text(message.text).textSelection(.enabled)
            ForEach(Array((message.sources ?? []).enumerated()),id:\.offset) { _,source in
                if let url=URL(string:source.url),["https","http"].contains(url.scheme ?? "") { Link(source.title,destination:url).font(.footnote) }
            }
        }.frame(maxWidth:.infinity,alignment:.leading).padding().background(message.role=="user" ? Color.green.opacity(0.08) : Color(uiColor:.secondarySystemGroupedBackground),in:RoundedRectangle(cornerRadius:16))
    }
}

struct CompanionNotebookView: View {
    @State private var confirmReload=false
    @ObservedObject var store:SanjanaStore
    private func text(_ key:WritableKeyPath<CompanionNotebook,String>)->Binding<String> {
        Binding(get:{store.notebook[keyPath:key]},set:{value in var n=store.notebook;n[keyPath:key]=value;store.editNotebook(n)})
    }
    var body: some View {
        Form {
            Section("Your companion") {
                TextField("Name",text:text(\.name))
                Text("Character and personality");TextEditor(text:text(\.profile)).frame(minHeight:100)
                Text("Research instructions");TextEditor(text:text(\.instructions)).frame(minHeight:100)
                Text("Working memory");TextEditor(text:text(\.memory)).frame(minHeight:100)
                TextField("Time zone",text:text(\.timezone)).textInputAutocapitalization(.never)
                Stepper("Daily research hour: \(store.notebook.hour)",value:Binding(get:{store.notebook.hour},set:{var n=store.notebook;n.hour=$0;store.editNotebook(n)}),in:0...23)
                Text("Character images can be uploaded from desktop settings.").font(.caption)
                Button("Save companion") {Task {await store.saveNotebook()}}.disabled(store.busy)
                Button("Reload saved settings") {confirmReload=true}.disabled(store.busy)
                Text(store.status).font(.footnote)
            }
            Section("Research history") {
                Button("Refresh history") {Task {await store.loadHistory(reset:true)}}.disabled(store.busy)
                ForEach(store.history,id:\.seq) {entry in
                    VStack(alignment:.leading,spacing:8) {
                        Text(Date(timeIntervalSince1970:entry.foundAt/1000).formatted()).font(.caption)
                        Text(entry.document.text).textSelection(.enabled)
                        ForEach(Array((entry.document.sources ?? []).enumerated()),id:\.offset) {_,source in
                            if let url=URL(string:source.url),["https","http"].contains(url.scheme ?? "") {Link(source.title,destination:url)}
                        }
                    }
                }
                if store.historyBefore != nil {Button("Older history") {Task {await store.loadHistory()}}.disabled(store.busy)}
            }
        }.navigationTitle("Companion settings")
        .confirmationDialog("Replace this phone draft with the saved companion settings?",isPresented:$confirmReload,titleVisibility:.visible) {
            Button("Replace phone draft",role:.destructive) {Task {await store.reloadNotebook()}}
        }
    }
}
