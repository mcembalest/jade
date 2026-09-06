import AppKit

final class Connection: NSObject, NSApplicationDelegate {
    let support = FileManager.default.homeDirectoryForCurrentUser.appendingPathComponent("Library/Application Support/JaDE")
    var process: Process?
    var item: NSStatusItem!
    var scopes: [URL] = []
    func applicationDidFinishLaunching(_ notification: Notification) {
        item = NSStatusBar.system.statusItem(withLength:NSStatusItem.variableLength)
        item.button?.title = "JaDE"
        let menu = NSMenu()
        menu.addItem(withTitle:"Choose writing folder or repo…",action:#selector(choose),keyEquivalent:"").target = self
        menu.addItem(withTitle:"Restart connection",action:#selector(restart),keyEquivalent:"").target = self
        menu.addItem(.separator())
        menu.addItem(withTitle:"Quit Mac connection",action:#selector(quit),keyEquivalent:"").target = self
        item.menu = menu
        restoreScopes()
        restart()
        if CommandLine.arguments.contains("--choose") { choose() }
    }
    func applicationShouldHandleReopen(_ sender:NSApplication,hasVisibleWindows flag:Bool)->Bool { choose();return true }
    func restoreScopes() {
        let bookmarks = UserDefaults.standard.dictionary(forKey:"folders") as? [String:Data] ?? [:]
        for data in bookmarks.values {
            var stale = false
            if let url = try? URL(resolvingBookmarkData:data,options:.withSecurityScope,relativeTo:nil,bookmarkDataIsStale:&stale) {
                if url.startAccessingSecurityScopedResource() { scopes.append(url) }
            }
        }
    }
    @objc func choose() {
        NSApp.activate(ignoringOtherApps:true)
        let panel = NSOpenPanel()
        panel.title = "Allow JaDE to edit a folder"
        panel.message = "Choose your writing folder or repo. JaDE on your iPhone will be able to read and edit text files here while this Mac is awake."
        panel.prompt = "Allow this folder"
        panel.canChooseFiles = false; panel.canChooseDirectories = true; panel.allowsMultipleSelection = false
        panel.directoryURL = FileManager.default.homeDirectoryForCurrentUser.appendingPathComponent("Documents/first")
        guard panel.runModal() == .OK, let url = panel.url else { return }
        do {
            let real = url.resolvingSymlinksInPath()
            guard real.path != "/" && real != FileManager.default.homeDirectoryForCurrentUser else { throw NSError(domain:"Choose a specific folder, not your whole disk or home folder",code:1) }
            if url.startAccessingSecurityScopedResource() { scopes.append(url) }
            var bookmarks = UserDefaults.standard.dictionary(forKey:"folders") as? [String:Data] ?? [:]
            bookmarks[real.path] = try url.bookmarkData(options:.withSecurityScope,includingResourceValuesForKeys:nil,relativeTo:nil)
            UserDefaults.standard.set(bookmarks,forKey:"folders")
            let config = support.appendingPathComponent("remote.json")
            var object = try JSONSerialization.jsonObject(with:Data(contentsOf:config)) as! [String:Any]
            var roots = object["roots"] as? [[String:String]] ?? []
            if !roots.contains(where:{$0["path"]==real.path}) { roots.append(["id":UUID().uuidString,"name":real.lastPathComponent,"path":real.path]) }
            object["roots"] = roots
            try JSONSerialization.data(withJSONObject:object,options:.prettyPrinted).write(to:config,options:.atomic)
            try FileManager.default.setAttributes([.posixPermissions:0o600],ofItemAtPath:config.path)
            restart()
        } catch {
            let alert = NSAlert();alert.messageText="Could not enable folder";alert.informativeText=error.localizedDescription;alert.runModal()
        }
    }
    @objc func restart() {
        if let old = process, old.isRunning { old.terminate();old.waitUntilExit() }
        let p = Process()
        p.executableURL = URL(fileURLWithPath:Bundle.main.object(forInfoDictionaryKey:"JaDEPython") as! String)
        p.arguments = [support.appendingPathComponent("remote/bridge.py").path]
        let log = support.appendingPathComponent("remote-app.log")
        if !FileManager.default.fileExists(atPath:log.path) { FileManager.default.createFile(atPath:log.path,contents:nil,attributes:[.posixPermissions:0o600]) }
        p.standardOutput = try? FileHandle(forWritingTo:log); p.standardError = p.standardOutput
        p.terminationHandler = { [weak self] child in
            DispatchQueue.main.asyncAfter(deadline:.now()+5) {
                if self?.process === child { self?.restart() }
            }
        }
        process = p
        do { try p.run() } catch { item.button?.title="JaDE disconnected" }
    }
    @objc func quit() { process?.terminationHandler=nil;process?.terminate();NSApp.terminate(nil) }
}
let app = NSApplication.shared
let delegate = Connection()
app.delegate = delegate
app.setActivationPolicy(.accessory)
app.run()
