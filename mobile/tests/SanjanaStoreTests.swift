import Foundation

final class SanjanaProtocol: URLProtocol {
    static var chats=0
    static var lost=false
    static var replies:[String:[String:Any]]=[:]
    static var state:[String:Any]=["enabled":true,"messages":[["id":"desktop","role":"assistant","text":"Desktop conversation"]],"pending":[["text":"A discovery","sources":[["title":"Source","url":"https://example.com"]]]],"next":9999999999999.0]
    override class func canInit(with request:URLRequest)->Bool { true }
    override class func canonicalRequest(for request:URLRequest)->URLRequest { request }
    override func startLoading() {
        var body:[String:Any]=[:]
        if request.httpMethod=="POST" {
            let stream=request.httpBodyStream!;stream.open();defer {stream.close()}
            var data=Data(),buffer=[UInt8](repeating:0,count:1024)
            while stream.hasBytesAvailable { let count=stream.read(&buffer,maxLength:buffer.count);if count<=0{break};data.append(buffer,count:count) }
            let value=try! JSONSerialization.jsonObject(with:data) as! [String:Any]
            if let action=value["companion"] as? [String:Any],action["action"] as? String=="chat" {
                Self.chats += 1
                Self.state["messages"]=[["id":"phone","role":"user","text":action["message"] as! String],["id":"reply","role":"assistant","text":"Same Sanjana"]]
            }
            Self.replies[value["id"] as! String]=Self.state
            if Self.lost { Self.lost=false;client?.urlProtocol(self,didFailWithError:URLError(.networkConnectionLost));return }
        } else {
            let id=URLComponents(url:request.url!,resolvingAgainstBaseURL:false)!.queryItems!.first!.value!
            body=["result":["companion":Self.replies[id]!]]
        }
        client?.urlProtocol(self,didReceive:HTTPURLResponse(url:request.url!,statusCode:200,httpVersion:nil,headerFields:nil)!,cacheStoragePolicy:.notAllowed)
        client?.urlProtocol(self,didLoad:try! JSONSerialization.data(withJSONObject:body));client?.urlProtocolDidFinishLoading(self)
    }
    override func stopLoading() {}
}
@main struct SanjanaStoreTests {
    @MainActor static func main() async throws {
        func check(_ value:Bool,_ message:String) { precondition(value,message) }
        let directory=FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        defer { try? FileManager.default.removeItem(at:directory) }
        let config=URLSessionConfiguration.ephemeral;config.protocolClasses=[SanjanaProtocol.self]
        let session=URLSession(configuration:config)
        let pair={Pairing(endpoint:"https://test.invalid",token:"test") as Pairing?}
        var store=SanjanaStore(directory:directory,session:session,pairing:pair)
        await store.refresh();check(store.state?.messages?.first?.text=="Desktop conversation","shared history")
        check(store.state?.pending?.count==1,"shared discoveries")
        store.edit("Hello")
        SanjanaProtocol.lost=true;await store.send();check(store.pending,"lost reply retains submitted identity")
        store.edit("newer draft")
        store=SanjanaStore(directory:directory,session:session,pairing:pair)
        await store.refresh()
        check(!store.pending && SanjanaProtocol.chats==1,"refresh must never resend accepted chat")
        check(store.draft=="newer draft","keep later typing")
        check(store.state?.messages?.last?.text=="Same Sanjana","shared reply")
        store=SanjanaStore(directory:directory,pairing:{nil})
        await store.refresh();check(store.state?.messages?.last?.text=="Same Sanjana","offline cache")
        let corrupt=directory.appendingPathComponent("corrupt");try FileManager.default.createDirectory(at:corrupt,withIntermediateDirectories:true)
        try Data("broken".utf8).write(to:corrupt.appendingPathComponent("sanjana.json"))
        let damaged=SanjanaStore(directory:corrupt,pairing:{nil});damaged.edit("replace")
        check(damaged.storageError != nil,"corrupt cache protected")
        check(try String(contentsOf:corrupt.appendingPathComponent("sanjana.json"),encoding:.utf8)=="broken","original retained")
        print("PASS: shared history/discoveries, lost-reply recovery without resend, newer drafts, offline cache, corrupt-cache protection")
    }
}
