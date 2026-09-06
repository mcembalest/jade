import Foundation

@main struct SyncStoreTests {
    @MainActor static func main() async throws {
        func expect(_ condition: @autoclosure () -> Bool, _ message: String) throws {
            if !condition() { throw SyncFailure(message: "TEST FAILED: " + message) }
        }
        let directory=FileManager.default.temporaryDirectory.appendingPathComponent("jade-swift-test-"+UUID().uuidString)
        defer {try? FileManager.default.removeItem(at:directory)}
        let endpoint=ProcessInfo.processInfo.environment["JADE_TEST_SYNC_URL"] ?? "http://127.0.0.1:8799"
        let pairing=Pairing(endpoint:endpoint,token:"local-test-key-not-for-production-123456")
        let path="phone-"+UUID().uuidString+".md"
        var store=SyncStore(directory:directory)
        try store.create(path,content:"offline on iPhone")
        store.edit(path,content:"survives closing the app")
        try expect(store.document(path)?.dirty == true,"offline note must be pending")
        store=SyncStore(directory:directory)
        try expect(store.document(path)?.content == "survives closing the app","local content must survive restart")
        try store.configure(pairing,allowLocalhost:true)
        await store.sync()
        try expect(store.document(path)?.dirty == false,"upload should be acknowledged")
        try expect(store.document(path)?.status == "Uploaded · Mac pending","must not claim Mac sync before receipt")
        func post<T: Encodable>(_ route:String,_ body:T) async throws -> [String:Any] {
            var req=URLRequest(url:URL(string:endpoint+route)!);req.httpMethod="POST";req.httpBody=try JSONEncoder().encode(body);req.setValue("Bearer "+pairing.token,forHTTPHeaderField:"Authorization");req.setValue("application/json",forHTTPHeaderField:"Content-Type")
            let (data,response)=try await URLSession.shared.data(for:req)
            try expect((response as? HTTPURLResponse)?.statusCode == 200,"test server request failed")
            return try JSONSerialization.jsonObject(with:data) as! [String:Any]
        }
        _ = try await post("/v1/ack",["path":path,"deviceId":"mac","revision":store.document(path)!.baseRevision])
        await store.sync()
        try expect(store.document(path)?.status == "Synced with Mac","Mac receipt should change status")
        let base=store.document(path)!.baseRevision
        store.edit(path,content:"offline iPhone conflict")
        _ = try await post("/v1/files",Mutation(path:path,content:"concurrent Mac edit",baseRevision:base,deviceId:"mac"))
        await store.sync()
        try expect(store.document(path)?.conflict?.content == "concurrent Mac edit","incoming conflict must be retained")
        try expect(store.document(path)?.content == "offline iPhone conflict","local conflict must remain untouched")
        store=SyncStore(directory:directory)
        try expect(store.document(path)?.conflict != nil,"conflict survives restart")
        try store.configure(pairing,allowLocalhost:true)
        try store.keepBoth(path)
        try expect(store.document(path)?.content == "concurrent Mac edit","incoming version becomes original")
        try expect(store.documents.contains{$0.content == "offline iPhone conflict" && $0.path != path},"conflict copy preserves local text")
        await store.sync()
        // Preserve an upload in the outbox, commit it remotely, then restart before receiving the response.
        var docs=store.documents
        let i=docs.firstIndex{$0.path==path}!
        let mutation=Mutation(path:path,content:"lost response",baseRevision:docs[i].baseRevision)
        docs[i].content=mutation.content;docs[i].pending=mutation
        try JSONEncoder().encode(docs).write(to:directory.appendingPathComponent("vault.json"),options:.atomic)
        _ = try await post("/v1/files",mutation)
        store=SyncStore(directory:directory);try store.configure(pairing,allowLocalhost:true);await store.sync()
        try expect(store.document(path)?.conflict == nil && store.document(path)?.pending == nil && store.document(path)?.baseContent == "lost response","lost response retry must recover without conflict")
        let corrupt=directory.appendingPathComponent("corrupt");try FileManager.default.createDirectory(at:corrupt,withIntermediateDirectories:true)
        let bad=corrupt.appendingPathComponent("vault.json");try Data("broken-json".utf8).write(to:bad)
        let broken=SyncStore(directory:corrupt);try expect(broken.storageError != nil,"corruption must be visible");try expect(!broken.persist(),"must not overwrite corrupt local store")
        let retained = try Data(contentsOf:bad)
        try expect(String(data:retained,encoding:.utf8)=="broken-json","corrupt file must remain recoverable")
        print("PASS: Swift offline restart, upload status, Mac receipts, conflicts, lost-response retry, and corrupt-store protection")
    }
}
