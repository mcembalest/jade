import Foundation
import Combine

struct SanjanaSource: Codable { var title:String;var url:String }
struct SanjanaMessage: Codable {
    var id:String?;var role:String?;var text:String;var sources:[SanjanaSource]?;var foundAt:Double?;var proactive:Bool?
}
struct CompanionNotebook: Codable, Equatable {
    var name="";var profile="";var instructions="";var memory="";var timezone="America/New_York";var hour=20;var avatar="";var revision=""
}
struct CompanionHistoryEntry: Codable { var seq:Int;var kind:String;var foundAt:Double;var document:SanjanaMessage }
struct CompanionHistory: Codable { var entries:[CompanionHistoryEntry];var before:Int? }
struct SanjanaState: Codable {
    var notebook:CompanionNotebook?
    var messages:[SanjanaMessage]?;var pending:[SanjanaMessage]?;var enabled:Bool
    var paused:Bool?;var profile:String?;var providerStatus:String?
    var next:Double?;var researchNext:Double?;var researchChecked:Double?;var researchError:String?;var seen:String?
}
struct SanjanaAction: Codable { var action:String;var message:String?;var enabled:Bool?;var seen:String?;var paused:Bool?;var notebook:CompanionNotebook?;var baseRevision:String?;var id:String? }
struct SanjanaRequest: Codable { var id=UUID().uuidString;var action="companion";var companion:SanjanaAction? }
private struct SanjanaCache: Codable {
    var notebookDraft:CompanionNotebook?;var history:[CompanionHistoryEntry]?;var historyBefore:Int?
    var state:SanjanaState?;var draft="";var pending:SanjanaRequest?;var submittedAt:Date?
}

@MainActor final class SanjanaStore: ObservableObject {
    @Published private var cache=SanjanaCache()
    @Published var busy=false
    @Published var status="Pair with Cloudflare to see your companion’s updates."
    @Published var storageError:String?
    private let disk:URL
    private let session:URLSession
    private let pairing:()->Pairing?
    private let allowLocalhost:Bool
    private var damaged=false
    var state:SanjanaState? { cache.state }
    var notebook:CompanionNotebook { cache.notebookDraft ?? cache.state?.notebook ?? CompanionNotebook() }
    var history:[CompanionHistoryEntry] { cache.history ?? [] }
    var historyBefore:Int? { cache.historyBefore }
    func editNotebook(_ value:CompanionNotebook) { guard !damaged else { return };cache.notebookDraft=value;do { try persist() } catch { status=error.localizedDescription } }
    func reloadNotebook() async {
        guard !busy,!damaged else {return};busy=true;defer{busy=false}
        do {let state=try await request();cache.state=state;cache.notebookDraft=nil;try persist();status="Saved companion settings loaded."}
        catch {status="Phone draft kept. "+error.localizedDescription}
    }
    func saveNotebook() async {
        guard !busy,!damaged else { return };busy=true;defer {busy=false}
        let snapshot=notebook
        do {
            cache.state=try await request(SanjanaAction(action:"notebook",notebook:snapshot,baseRevision:snapshot.revision,id:UUID().uuidString))
            if cache.notebookDraft==snapshot {cache.notebookDraft=nil}
            else if cache.notebookDraft?.revision==snapshot.revision,let revision=cache.state?.notebook?.revision {cache.notebookDraft?.revision=revision}
            try persist();status="Companion saved for all devices."
        } catch {status="Settings not saved. Your phone draft is kept. "+error.localizedDescription}
    }
    func loadHistory(reset:Bool=false) async {
        guard !busy,!damaged,let pair=pairing() else {return};busy=true;defer {busy=false}
        do {
            var parts=URLComponents(string:pair.endpoint+"/v1/companion")!
            guard parts.scheme=="https" || (allowLocalhost && parts.scheme=="http" && parts.host=="127.0.0.1") else {throw SyncFailure(message:"Invalid endpoint")}
            parts.queryItems=[URLQueryItem(name:"history",value:"1"),URLQueryItem(name:"kind",value:"research")]
            if !reset,let cursor=cache.historyBefore {parts.queryItems?.append(URLQueryItem(name:"before",value:String(cursor)))}
            var req=URLRequest(url:parts.url!);req.timeoutInterval=20;req.setValue("Bearer "+pair.token,forHTTPHeaderField:"Authorization")
            let(data,response)=try await session.data(for:req)
            guard (response as? HTTPURLResponse)?.statusCode==200 else {throw SyncFailure(message:"History unavailable")}
            let page=try JSONDecoder().decode(CompanionHistory.self,from:data)
            let existing=reset ? [] : cache.history ?? [];let ids=Set(existing.map { $0.seq })
            cache.history=existing+page.entries.filter { !ids.contains($0.seq) };cache.historyBefore=page.before;try persist()
        }catch{status="Saved history is kept. "+error.localizedDescription}
    }
    var draft:String { cache.draft }
    var pending:Bool { cache.pending != nil }
    init(directory:URL?=nil,session:URLSession = .shared,pairing:@escaping ()->Pairing?,allowLocalhost:Bool=false) {
        self.session=session;self.pairing=pairing;self.allowLocalhost=allowLocalhost
        var folder=directory ?? FileManager.default.urls(for:.applicationSupportDirectory,in:.userDomainMask)[0].appendingPathComponent("JaDE")
        #if DEBUG
        if let id=ProcessInfo.processInfo.environment["JADE_UI_TEST_ID"],UUID(uuidString:id) != nil { folder=folder.appendingPathComponent("UITests/"+id) }
        #endif
        disk=folder.appendingPathComponent("sanjana.json")
        do {
            try FileManager.default.createDirectory(at:folder,withIntermediateDirectories:true)
            if FileManager.default.fileExists(atPath:disk.path) { cache=try JSONDecoder().decode(SanjanaCache.self,from:Data(contentsOf:disk)) }
            #if DEBUG
            if !FileManager.default.fileExists(atPath:disk.path),ProcessInfo.processInfo.environment["JADE_OFFLINE_UI_TEST"]=="1",let fixture=ProcessInfo.processInfo.environment["JADE_SANJANA_FIXTURE"] {
                cache.state=try JSONDecoder().decode(SanjanaState.self,from:Data(fixture.utf8))
            }
            #endif
        } catch { damaged=true;storageError="Saved conversation could not be read. Original storage is preserved." }
    }
    private func persist() throws {
        guard !damaged else { throw SyncFailure(message:storageError ?? "Saved conversation needs recovery") }
        do {
            #if os(iOS)
            try JSONEncoder().encode(cache).write(to:disk,options:[.atomic,.completeFileProtectionUntilFirstUserAuthentication])
            #else
            try JSONEncoder().encode(cache).write(to:disk,options:.atomic)
            #endif
            let handle=try FileHandle(forWritingTo:disk);defer { try? handle.close() };try handle.synchronize();storageError=nil
        } catch { storageError="Phone save failed. Keep JaDE open and copy your message.";throw error }
    }
    func edit(_ text:String) { guard !damaged else { return };cache.draft=text;do { try persist() } catch { status=error.localizedDescription } }
    private func request(_ action:SanjanaAction?=nil) async throws -> SanjanaState {
        guard let pair=pairing(),let url=URL(string:pair.endpoint+"/v1/companion"),url.scheme=="https" || (allowLocalhost && url.scheme=="http" && url.host=="127.0.0.1") else { throw SyncFailure(message:"Pair JaDE in Notes settings first.") }
        var request=URLRequest(url:url);request.timeoutInterval=20
        request.setValue("Bearer "+pair.token,forHTTPHeaderField:"Authorization")
        if let action { request.httpMethod="POST";request.httpBody=try JSONEncoder().encode(action);request.setValue("application/json",forHTTPHeaderField:"Content-Type") }
        let (data,response)=try await session.data(for:request)
        guard (response as? HTTPURLResponse)?.statusCode==200 else {
            if (response as? HTTPURLResponse)?.statusCode==409 {throw SyncFailure(message:"Settings changed on another device. Copy any phone edits you want to keep, then reload saved settings before saving again.")}
            throw SyncFailure(message:"Cloud updates unavailable. Saved updates are kept.")
        }
        return try JSONDecoder().decode(SanjanaState.self,from:data)
    }
    func refresh() async {
        guard !busy,!damaged else { return };busy=true;defer { busy=false }
        do {
            // Reads only. Legacy drafts and unconfirmed requests remain on disk.
            cache.state=try await request();try persist()
            status="Shared through Cloudflare · updates checked just now"
        } catch { status="Offline · showing saved updates. "+error.localizedDescription }
    }
    func act(_ action:SanjanaAction) async {
        guard !busy,!damaged,action.action=="settings" else { return };busy=true;defer { busy=false }
        do { cache.state=try await request(action);try persist();status="Research setting saved for all devices." }
        catch { status=error.localizedDescription }
    }
}
