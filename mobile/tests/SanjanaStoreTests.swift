import Foundation

final class SanjanaProtocol: URLProtocol {
    static var reads=0, writes=0
    static var offline=false
    static var state:[String:Any]=["enabled":true,"paused":false,"messages":[["id":"desktop","role":"assistant","text":"Shared update","proactive":true]],"pending":[["text":"A discovery","sources":[["title":"Source","url":"https://example.com"]]]],"next":0.0,"researchNext":0.0]
    override class func canInit(with request:URLRequest)->Bool { true }
    override class func canonicalRequest(for request:URLRequest)->URLRequest { request }
    override func startLoading() {
        precondition(request.url!.path=="/v1/companion","must bypass Mac relay")
        if Self.offline { client?.urlProtocol(self,didFailWithError:URLError(.notConnectedToInternet));return }
        if request.httpMethod=="POST" { Self.writes += 1;Self.state["paused"]=true } else { Self.reads += 1 }
        client?.urlProtocol(self,didReceive:HTTPURLResponse(url:request.url!,statusCode:200,httpVersion:nil,headerFields:nil)!,cacheStoragePolicy:.notAllowed)
        client?.urlProtocol(self,didLoad:try! JSONSerialization.data(withJSONObject:Self.state));client?.urlProtocolDidFinishLoading(self)
    }
    override func stopLoading() {}
}
@main struct SanjanaStoreTests {
    @MainActor static func main() async throws {
        func check(_ value:Bool,_ message:String) { precondition(value,message) }
        let directory=FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        defer { try? FileManager.default.removeItem(at:directory) }
        try FileManager.default.createDirectory(at:directory,withIntermediateDirectories:true)
        let legacy:[String:Any]=["draft":"old draft","pending":["id":"legacy","action":"companion","companion":["action":"chat","message":"old message"]]]
        try JSONSerialization.data(withJSONObject:legacy).write(to:directory.appendingPathComponent("sanjana.json"))
        let config=URLSessionConfiguration.ephemeral;config.protocolClasses=[SanjanaProtocol.self]
        let session=URLSession(configuration:config)
        let pair={Pairing(endpoint:"https://test.invalid",token:"test") as Pairing?}
        var store=SanjanaStore(directory:directory,session:session,pairing:pair)
        for _ in 0..<3 { await store.refresh() }
        check(SanjanaProtocol.writes==0 && SanjanaProtocol.reads==3,"overdue clocks never cause research, delivery or legacy replay")
        check(store.state?.messages?.first?.text=="Shared update" && store.state?.pending?.count==1,"cloud history and queue")
        check(store.draft=="old draft" && store.pending,"preserve legacy draft and receipt")
        await store.act(SanjanaAction(action:"research"));check(SanjanaProtocol.writes==0,"reject old manual research")
        await store.act(SanjanaAction(action:"settings",paused:true));check(store.state?.paused==true && SanjanaProtocol.writes==1,"explicit shared pause")
        SanjanaProtocol.offline=true
        store=SanjanaStore(directory:directory,session:session,pairing:pair)
        await store.refresh();check(store.state?.messages?.first?.text=="Shared update","offline cache")
        check(store.draft=="old draft" && store.pending,"legacy data survives cloud refresh and restart")
        let corrupt=directory.appendingPathComponent("corrupt");try FileManager.default.createDirectory(at:corrupt,withIntermediateDirectories:true)
        try Data("broken".utf8).write(to:corrupt.appendingPathComponent("sanjana.json"))
        let damaged=SanjanaStore(directory:corrupt,pairing:{nil});damaged.edit("replace")
        check(damaged.storageError != nil,"corrupt cache protected")
        check(try String(contentsOf:corrupt.appendingPathComponent("sanjana.json"),encoding:.utf8)=="broken","original retained")
        print("PASS: read-only refresh, no Mac relay, explicit shared pause, legacy preservation, offline restart, corrupt-cache protection")
    }
}
