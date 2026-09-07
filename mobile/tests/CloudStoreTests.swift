import Foundation

final class DownloadProtocol: URLProtocol {
    static var attempts:[String:Int]=[:]
    override class func canInit(with request:URLRequest)->Bool { true }
    override class func canonicalRequest(for request:URLRequest)->URLRequest { request }
    override func startLoading() {
        let url=request.url!,path=URLComponents(url:url,resolvingAgainstBaseURL:false)?.queryItems?.first?.value ?? "listing"
        Self.attempts[path,default:0] += 1
        if path=="b.py",Self.attempts[path]==1 { client?.urlProtocol(self,didFailWithError:URLError(.notConnectedToInternet));return }
        let body:[String:Any]
        if path=="listing" { body=["files":["a.py","b.py","c.py"].map{["path":$0,"revision":"r-"+$0,"writer":"mac","updatedAt":"now","bytes":5,"macRevision":"","macIssue":""]}] }
        else { body=["file":["path":path,"revision":"r-"+path,"content":"saved "+path]] }
        client?.urlProtocol(self,didReceive:HTTPURLResponse(url:url,statusCode:200,httpVersion:nil,headerFields:nil)!,cacheStoragePolicy:.notAllowed)
        client?.urlProtocol(self,didLoad:try! JSONSerialization.data(withJSONObject:body));client?.urlProtocolDidFinishLoading(self)
    }
    override func stopLoading() {}
}

@main struct CloudStoreTests {
    @MainActor static func main() async throws {
        func expect(_ condition:@autoclosure ()->Bool,_ message:String) throws { if !condition() { throw SyncFailure(message:"TEST FAILED: "+message) } }
        let dir=FileManager.default.temporaryDirectory.appendingPathComponent("jade-cloud-test-"+UUID().uuidString)
        defer { try? FileManager.default.removeItem(at:dir) }
        let endpoint=ProcessInfo.processInfo.environment["JADE_TEST_SYNC_URL"] ?? "http://127.0.0.1:8799"
        let pair=Pairing(endpoint:endpoint,token:"local-test-key-not-for-production-123456")
        let project="swift-"+UUID().uuidString
        func post(_ suffix:String,_ body:[String:Any],agent:Bool=true) async throws -> [String:Any] {
            var req=URLRequest(url:URL(string:endpoint+"/v1/projects/"+project+suffix)!)
            req.httpMethod="POST";req.httpBody=try JSONSerialization.data(withJSONObject:body)
            req.setValue("Bearer "+(agent ? "agent-test-secret":pair.token),forHTTPHeaderField:"Authorization");req.setValue("application/json",forHTTPHeaderField:"Content-Type")
            let (data,r)=try await URLSession.shared.data(for:req);try expect((r as? HTTPURLResponse)?.statusCode==200,"fixture request")
            return try JSONSerialization.jsonObject(with:data) as! [String:Any]
        }
        _ = try await post("",["name":"Swift test","enabled":true])
        _ = try await post("/file",["path":"code.py","content":"Mac version","baseRevision":"","mutationId":"original"])
        var store=CloudStore(directory:dir);try store.configure(pair,allowLocalhost:true);await store.refresh()
        try await store.fetch(project,"code.py")
        store.edit(project,"code.py",content:"phone draft")
        // Unconfigured replacement simulates no network: submission persists before failing.
        store=CloudStore(directory:dir);await store.submit(project,"code.py")
        try expect(store.document(project,"code.py")?.pending != nil,"explicit submission must queue locally")
        let queued=store.document(project,"code.py")!.pending!
        store.edit(project,"code.py",content:"newer unsubmitted draft")
        _ = try await post("/file",["path":queued.path,"content":queued.content,"baseRevision":queued.baseRevision,"mutationId":queued.mutationId],agent:false)
        // Restart after cloud acceptance but before the phone sees the reply.
        store=CloudStore(directory:dir);try store.configure(pair,allowLocalhost:true);await store.refresh()
        try expect(store.document(project,"code.py")?.pending==nil,"lost reply must replay safely")
        try expect(store.document(project,"code.py")?.baseContent=="phone draft","only submitted snapshot accepted")
        try expect(store.document(project,"code.py")?.content=="newer unsubmitted draft","new typing preserved")
        try expect(store.document(project,"code.py")?.dirty==true,"new typing not automatically submitted")
        do { try await store.fetch(project,"code.py");throw SyncFailure(message:"TEST FAILED: replaced dirty draft") } catch let e as SyncFailure { try expect(!e.message.hasPrefix("TEST FAILED"),"fetch must protect draft") }
        await store.submit(project,"code.py")
        let revision=store.document(project,"code.py")!.revision
        try expect(store.document(project,"code.py")?.status=="Stored in Cloudflare · Mac pending","must distinguish cloud from Mac")
        _ = try await post("/ack",["path":"code.py","revision":revision,"applied":true]);await store.refresh()
        try expect(store.document(project,"code.py")?.status=="Stored in Cloudflare · applied on Mac","exact Mac receipt")
        store.edit(project,"code.py",content:"phone conflict")
        _ = try await post("/file",["path":"code.py","content":"another version","baseRevision":revision,"mutationId":"other"])
        await store.submit(project,"code.py")
        try expect(store.document(project,"code.py")?.conflict != nil,"cloud conflict retained")
        try expect(store.document(project,"code.py")?.content=="phone conflict","local draft preserved")
        try store.saveCopy(project,"code.py",as:"code-copy.py")
        try expect(store.document(project,"code-copy.py")?.content=="phone conflict","copy preserves draft")
        try await store.fetch(project,"code.py",replaceDraft:true)
        try expect(store.document(project,"code.py")?.content=="another version","explicit fetch accepted")
        let history=try await store.history(project,"code.py");try expect(history.count==4,"history includes accepted snapshots only")
        _ = try await post("/file",["path":"offline.py","content":"offline contents","baseRevision":"","mutationId":"offline-file"])
        store.edit(project,"code.py",content:"do not replace this draft")
        await store.downloadProject(project)
        try expect(store.document(project,"offline.py")?.content=="offline contents","bulk download includes unfetched content")
        try expect(store.document(project,"code.py")?.content=="do not replace this draft","bulk download preserves drafts")
        store=CloudStore(directory:dir)
        try expect(store.document(project,"offline.py")?.content=="offline contents","bulk contents survive offline restart")
        try store.configure(pair,allowLocalhost:true)
        _ = try await post("/file",["path":"code.py","content":"cloud changed again","baseRevision":"other","mutationId":"conflict-again"])
        await store.submit(project,"code.py")
        try store.resolve(project,"code.py",content:"combined result")
        try expect(store.document(project,"code.py")?.pending==nil,"resolution must not submit automatically")
        try expect(store.document(project,"code.py")?.revision=="conflict-again","resolution rebases on reviewed cloud revision")
        try expect(store.documents.contains(where:{$0.path.hasPrefix("Recovered drafts/") && $0.content=="do not replace this draft"}),"resolution preserves original draft")
        _ = try await post("/file",["path":"code.py","content":"changed during resolution","baseRevision":"conflict-again","mutationId":"resolution-race"])
        await store.submit(project,"code.py")
        try expect(store.document(project,"code.py")?.conflict != nil,"new cloud changes still conflict after resolution")
        try expect(store.document(project,"code.py")?.content=="combined result","racing resolution keeps merged draft")
        let corrupt=dir.appendingPathComponent("corrupt");try FileManager.default.createDirectory(at:corrupt,withIntermediateDirectories:true)
        try Data("broken".utf8).write(to:corrupt.appendingPathComponent("cloud-projects.json"))
        let bad=CloudStore(directory:corrupt);await bad.refresh();try expect(bad.storageError != nil,"corrupt cache preserved")
        let config=URLSessionConfiguration.ephemeral;config.protocolClasses=[DownloadProtocol.self]
        let downloadDir=dir.appendingPathComponent("download")
        var downloads=CloudStore(directory:downloadDir,session:URLSession(configuration:config))
        try downloads.configure(Pairing(endpoint:"https://offline.test",token:"test"))
        await downloads.downloadProject("download")
        try expect(downloads.offlineCount("download")==1,"interruption keeps completed download")
        downloads.edit("download","a.py",content:"offline edit")
        downloads=CloudStore(directory:downloadDir,session:URLSession(configuration:config))
        try downloads.configure(Pairing(endpoint:"https://offline.test",token:"test"))
        await downloads.downloadProject("download")
        try expect(downloads.offlineCount("download")==3,"bulk download resumes after restart")
        try expect(DownloadProtocol.attempts["a.py"]==1,"resume skips completed files")
        try expect(downloads.document("download","a.py")?.content=="offline edit","resume protects offline edits")
        print("PASS: cloud draft persistence, explicit submissions, lost reply, newer typing, Mac receipts, conflicts, history and corrupt-cache protection")
    }
}
