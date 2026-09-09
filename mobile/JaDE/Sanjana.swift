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
    @Environment(\.scenePhase) private var phase
    var body: some View {
        NavigationStack {
            ScrollView {
                VStack(alignment:.leading,spacing:20) {
                    VStack(spacing:6) {
                        SanjanaSprite(still:still)
                        Text("A LITTLE COMPANY").font(.caption.weight(.semibold)).tracking(2)
                        Text("Sanjana’s corner").font(.largeTitle.weight(.semibold))
                        Text("Fashion, rabbit holes, and something good in the city.").font(.subheadline).foregroundStyle(.secondary).multilineTextAlignment(.center)
                    }.frame(maxWidth:.infinity).padding(.vertical)
                    Text(store.storageError ?? store.status).font(.footnote).foregroundStyle(.secondary).accessibilityIdentifier("Sanjana status")
                    if store.busy { ProgressView() }
                    if store.state?.enabled==false {
                        Button("Show Sanjana on all devices") { Task { await store.act(SanjanaAction(action:"enabled",enabled:true)) } }.buttonStyle(.borderedProminent)
                    }
                    DisclosureGroup("Discoveries · \(store.state?.pending?.count ?? 0) pending") {
                        VStack(alignment:.leading,spacing:16) {
                            ForEach(Array((store.state?.pending ?? []).enumerated()),id:\.offset) { _,message in row(message) }
                            if let error=store.state?.researchError,!error.isEmpty { Text(error).font(.footnote) }
                            Button("Look for discoveries") { Task { await store.act(SanjanaAction(action:"research")) } }.disabled(store.busy || store.state?.enabled != true)
                            Text("Uses the same hourly limit and 8pm daily update as desktop Sanjana.").font(.caption).foregroundStyle(.secondary)
                        }.padding(.top,8)
                    }
                    Text("Updates from Sanjana").font(.title2.weight(.semibold))
                    ForEach(Array((store.state?.messages ?? []).filter { $0.proactive == true }.reversed().enumerated()),id:\.offset) { _,message in row(message) }
                    if !(store.state?.messages ?? []).contains(where:{$0.proactive == true}) { Text("Her updates will appear here as she finds things to share.").foregroundStyle(.secondary) }
                    Text("Sanjana looks for discoveries while this tab or her desktop window is open. She shares a daily update around 8pm, using your Mac’s clock. Keep your Mac awake; saved updates are available offline.").font(.caption).foregroundStyle(.secondary)
                }.padding()
            }.background(Color(uiColor:.systemGroupedBackground)).navigationTitle("Sanjana").navigationBarTitleDisplayMode(.inline)
                .toolbar { Menu {
                    Button("Refresh updates") { Task { await store.refresh() } }.disabled(store.busy)
                    Toggle("Still animation",isOn:$still)
                    Button("Hide Sanjana on all devices") { Task { await store.act(SanjanaAction(action:"enabled",enabled:false)) } }.disabled(store.busy)
                } label: { Image(systemName:"ellipsis.circle") } }
                .task(id:phase) {
                    guard phase == .active else { return }
                    await store.heartbeat()
                    while !Task.isCancelled {
                        do { try await Task.sleep(for:.seconds(30)) } catch { return }
                        await store.heartbeat()
                    }
                }
        }
    }
    private func row(_ message:SanjanaMessage) -> some View {
        VStack(alignment:.leading,spacing:8) {
            Text(message.role=="user" ? "You" : "Sanjana").font(.caption.weight(.semibold)).foregroundStyle(.secondary)
            if let date=message.foundAt { Text(Date(timeIntervalSince1970:date/1000).formatted()).font(.caption).foregroundStyle(.secondary) }
            Text(message.text).textSelection(.enabled)
            ForEach(Array((message.sources ?? []).enumerated()),id:\.offset) { _,source in
                if let url=URL(string:source.url),["https","http"].contains(url.scheme ?? "") { Link(source.title,destination:url).font(.footnote) }
            }
        }.frame(maxWidth:.infinity,alignment:.leading).padding().background(message.role=="user" ? Color.green.opacity(0.08) : Color(uiColor:.secondarySystemGroupedBackground),in:RoundedRectangle(cornerRadius:16))
    }
}
