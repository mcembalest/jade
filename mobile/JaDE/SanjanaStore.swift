import Foundation
import Combine

struct SanjanaSource: Codable { var title:String;var url:String }
struct SanjanaMessage: Codable {
    var id:String?;var role:String?;var text:String;var sources:[SanjanaSource]?;var foundAt:Double?;var proactive:Bool?
}
struct SanjanaState: Codable {
    var messages:[SanjanaMessage]?;var pending:[SanjanaMessage]?;var enabled:Bool
    var next:Double?;var researchNext:Double?;var researchChecked:Double?;var researchError:String?;var seen:String?
}
struct SanjanaAction: Codable { var action:String;var message:String?;var enabled:Bool?;var seen:String? }
struct SanjanaRequest: Codable { var id=UUID().uuidString;var action="companion";var companion:SanjanaAction? }
private struct SanjanaCache: Codable {
    var state:SanjanaState?;var draft="";var pending:SanjanaRequest?;var submittedAt:Date?
}

@MainActor final class SanjanaStore: ObservableObject {
    @Published private var cache=SanjanaCache()
    @Published var busy=false
    @Published var status="Connect to your Mac to see Sanjana’s updates."
    @Published var storageError:String?
    private let disk:URL
    private let session:URLSession
    private let pairing:()->Pairing?
    private let allowLocalhost:Bool
    private var damaged=false
    private var connected=false
    var state:SanjanaState? { cache.state }
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
    private struct Envelope: Decodable {
        struct Result: Decodable { var companion:SanjanaState?;var error:String? }
        var result:Result?;var error:String?
    }
    private struct ReplyFailure: LocalizedError { var message:String;var errorDescription:String? { message } }
    private func http(_ path:String,body:SanjanaRequest?=nil) async throws -> Envelope {
        guard let pair=pairing(),let url=URL(string:pair.endpoint+path),url.scheme=="https" || (allowLocalhost && url.scheme=="http" && url.host=="127.0.0.1") else { throw SyncFailure(message:"Pair JaDE in Notes settings first.") }
        var request=URLRequest(url:url);request.timeoutInterval=20
        request.setValue("Bearer "+pair.token,forHTTPHeaderField:"Authorization")
        if let body { request.httpMethod="POST";request.httpBody=try JSONEncoder().encode(body);request.setValue("application/json",forHTTPHeaderField:"Content-Type") }
        let (data,response)=try await session.data(for:request)
        let result=try JSONDecoder().decode(Envelope.self,from:data)
        guard (response as? HTTPURLResponse)?.statusCode==200 else { throw SyncFailure(message:result.error ?? "Mac connection unavailable") }
        return result
    }
    private func poll(_ request:SanjanaRequest,seconds:Int) async throws -> SanjanaState {
        for _ in 0..<seconds {
            try Task.checkCancellation()
            let reply=try await http("/v1/remote/result?id="+request.id)
            if let result=reply.result {
                if let error=result.error { throw ReplyFailure(message:error) }
                if let state=result.companion { return state }
                throw SyncFailure(message:"Update the Mac connection to use Sanjana.")
            }
            try await Task.sleep(for:.seconds(1))
        }
        throw SyncFailure(message:"Mac has not replied yet. It may still be working; refresh to check. Your message is kept.")
    }
    private func request(_ action:SanjanaAction?=nil) async throws -> SanjanaState {
        let request=SanjanaRequest(companion:action)
        _ = try await http("/v1/remote/request",body:request)
        return try await poll(request,seconds:action?.action=="research" ? 110 : 25)
    }
    func refresh() async {
        guard !busy,!damaged else { return };busy=true;connected=false;defer { busy=false }
        do {
            // Refresh only checks a submitted ID; it never resends a chat message.
            if let pending=cache.pending {
                do { try accept(try await poll(pending,seconds:1),request:pending) }
                catch is ReplyFailure { cache.pending=nil;cache.submittedAt=nil;try persist() }
                catch { status="Previous message has no confirmed reply. Shared history is refreshed below; your draft is kept." }
            }
            cache.state=try await request();connected=true;try persist()
            status="Shared with desktop Sanjana · updates checked just now"
            if cache.state?.enabled==true,let next=cache.state?.next,next<=Date().timeIntervalSince1970*1000,!(cache.state?.pending ?? []).isEmpty {
                cache.state=try await request(SanjanaAction(action:"discover"));try persist()
            }
        } catch { status="Offline / Mac unavailable. Showing saved updates. "+error.localizedDescription }
    }
    private func accept(_ state:SanjanaState,request:SanjanaRequest) throws {
        cache.state=state
        if cache.draft==request.companion?.message { cache.draft="" }
        cache.pending=nil;cache.submittedAt=nil;try persist()
    }
    func send() async {
        guard !busy,!damaged,cache.pending==nil else { return }
        let message=cache.draft.trimmingCharacters(in:.whitespacesAndNewlines)
        guard !message.isEmpty,message.utf8.count<=8000 else { status="Write a message of up to 8,000 bytes.";return }
        busy=true;defer { busy=false }
        do {
            let request=SanjanaRequest(companion:SanjanaAction(action:"chat",message:cache.draft))
            cache.pending=request;cache.submittedAt=Date();try persist()
            status="Sending to your Mac · Sanjana is thinking…"
            _ = try await http("/v1/remote/request",body:request)
            try accept(try await poll(request,seconds:210),request:request)
            status="Conversation saved by Sanjana on your Mac."
        } catch let error as ReplyFailure {
            cache.pending=nil;cache.submittedAt=nil;try? persist();status=error.localizedDescription+" Your draft is kept."
        } catch { status=error.localizedDescription+" Refresh to check the conversation; this message will not be sent again automatically." }
    }
    func releaseUnconfirmed() {
        guard !busy else { return }
        if let started=cache.submittedAt,Date().timeIntervalSince(started)<240 { status="Sanjana may still be replying. Wait four minutes from sending, then check the conversation before trying again.";return }
        cache.pending=nil;cache.submittedAt=nil
        do { try persist();status="Draft retained. Check shared history before sending it again." } catch { status=error.localizedDescription }
    }
    func act(_ action:SanjanaAction) async {
        guard !busy,!damaged else { return };busy=true;defer { busy=false }
        status=action.action=="research" ? "Sanjana is looking for discoveries…" : "Checking Sanjana…"
        do { cache.state=try await request(action);try persist();status="Shared with desktop Sanjana · checked just now" }
        catch { status=error.localizedDescription }
    }
    func heartbeat() async {
        await refresh()
        guard !Task.isCancelled,connected,!busy,cache.state?.enabled==true,
              let next=cache.state?.researchNext,next<=Date().timeIntervalSince1970*1000,(cache.state?.pending ?? []).count<24 else { return }
        await act(SanjanaAction(action:"research"))
    }
}
