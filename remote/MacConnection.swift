import AppKit

final class Connection: NSObject, NSApplicationDelegate, NSMenuDelegate {
    let support = FileManager.default.homeDirectoryForCurrentUser.appendingPathComponent("Library/Application Support/JaDE")
    var process: Process?
    var item: NSStatusItem!
    var scopes: [URL] = []
    func applicationDidFinishLaunching(_ notification: Notification) {
        item = NSStatusBar.system.statusItem(withLength:NSStatusItem.variableLength)
        item.button?.title = "JaDE"
        let menu = NSMenu(); menu.delegate = self
        menu.addItem(withTitle:"Open JaDE in browser",action:#selector(openEditor),keyEquivalent:"").target = self
        menu.addItem(.separator())
        menu.addItem(withTitle:"Choose writing folder or repo…",action:#selector(choose),keyEquivalent:"").target = self
        let projects = NSMenuItem(title:"Cloud projects",action:nil,keyEquivalent:"")
        projects.submenu = NSMenu(); menu.addItem(projects)
        menu.addItem(withTitle:"Cloud connection status…",action:#selector(cloudStatus),keyEquivalent:"").target = self
        menu.addItem(withTitle:"Restart connection",action:#selector(restart),keyEquivalent:"").target = self
        menu.addItem(.separator())
        menu.addItem(withTitle:"Quit Mac connection",action:#selector(quit),keyEquivalent:"").target = self
        item.menu = menu
        restoreScopes()
        restart()
        if CommandLine.arguments.contains("--choose") { choose() }
    }
    func applicationShouldHandleReopen(_ sender:NSApplication,hasVisibleWindows flag:Bool)->Bool { choose();return true }
    @objc func openEditor() {
        // The installed desktop service uses this address (sync/install-mac.py).
        let url = URL(string:"http://127.0.0.1:7339")!
        if !NSWorkspace.shared.open(url) {
            NSApp.activate(ignoringOtherApps:true)
            let alert = NSAlert()
            alert.messageText = "Could not open JaDE in your browser"
            alert.informativeText = "Open http://127.0.0.1:7339 in your browser."
            alert.runModal()
        }
    }
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
            var roots = object["roots"] as? [[String:Any]] ?? []
            if !roots.contains(where:{($0["path"] as? String)==real.path}) { roots.append(["id":UUID().uuidString,"name":real.lastPathComponent,"path":real.path]) }
            object["roots"] = roots
            try JSONSerialization.data(withJSONObject:object,options:.prettyPrinted).write(to:config,options:.atomic)
            try FileManager.default.setAttributes([.posixPermissions:0o600],ofItemAtPath:config.path)
            restart()
        } catch {
            let alert = NSAlert();alert.messageText="Could not enable folder";alert.informativeText=error.localizedDescription;alert.runModal()
        }
    }
    func menuWillOpen(_ menu: NSMenu) {
        guard let submenu = menu.items.first(where:{$0.title=="Cloud projects"})?.submenu else { return }
        submenu.removeAllItems()
        guard let data=try? Data(contentsOf:support.appendingPathComponent("remote.json")),
              let config=try? JSONSerialization.jsonObject(with:data) as? [String:Any],
              let roots=config["roots"] as? [[String:Any]] else { return }
        for root in roots {
            guard let id=root["id"] as? String,let name=root["name"] as? String else { continue }
            let enabled=root["cloud"] as? Bool ?? false
            let entry=NSMenuItem(title:(enabled ? "Pause cloud sync: " : "Enable cloud copy: ")+name,action:#selector(toggleCloud(_:)),keyEquivalent:"")
            entry.target=self;entry.representedObject=id;submenu.addItem(entry)
        }
        if roots.isEmpty { let empty=NSMenuItem(title:"Choose a folder first",action:nil,keyEquivalent:"");empty.isEnabled=false;submenu.addItem(empty) }
    }
    @objc func toggleCloud(_ sender:NSMenuItem) {
        do {
            let url=support.appendingPathComponent("remote.json")
            var object=try JSONSerialization.jsonObject(with:Data(contentsOf:url)) as! [String:Any]
            var roots=object["roots"] as? [[String:Any]] ?? []
            guard let id=sender.representedObject as? String,let index=roots.firstIndex(where:{($0["id"] as? String)==id}) else { return }
            let enabled=roots[index]["cloud"] as? Bool ?? false
            NSApp.activate(ignoringOtherApps:true)
            let alert=NSAlert()
            alert.messageText=enabled ? "Pause cloud sync for this project?" : "Keep this project available through Cloudflare?"
            alert.informativeText=enabled ? "Cloud copies and history stay available for reading. New cloud submissions and Mac delivery pause after the connection checks this setting." : "Supported text files in this folder will be uploaded to your Cloudflare account. You can read and submit edits while your Mac is off. Cloudflare can read these contents. Hidden files, Git-ignored files, generated folders and key/certificate files are excluded. Limit: 2,000 files, 32 MB per project; 256 KB per file. Existing Mac-files editing stays separate."
            alert.addButton(withTitle:enabled ? "Pause sync" : "Enable cloud copy");alert.addButton(withTitle:"Cancel")
            guard alert.runModal() == .alertFirstButtonReturn else { return }
            roots[index]["cloud"] = !enabled;object["roots"]=roots
            try JSONSerialization.data(withJSONObject:object,options:.prettyPrinted).write(to:url,options:.atomic)
            try FileManager.default.setAttributes([.posixPermissions:0o600],ofItemAtPath:url.path)
        } catch { showError(error.localizedDescription) }
    }
    @objc func cloudStatus() {
        let file=support.appendingPathComponent("cloud-status.json")
        var text="No cloud project check yet. Enable a folder from the Cloud projects menu."
        if let data=try? Data(contentsOf:file),let status=try? JSONSerialization.jsonObject(with:data) as? [String:Any] {
            let configData=try? Data(contentsOf:support.appendingPathComponent("remote.json"))
            let config=configData.flatMap { try? JSONSerialization.jsonObject(with:$0) as? [String:Any] }
            let roots=config?["roots"] as? [[String:Any]] ?? []
            let states=status["projects"] as? [String:String] ?? [:]
            let lines=roots.compactMap { root -> String? in
                guard let id=root["id"] as? String,let name=root["name"] as? String,let state=states[id] else { return nil }
                return name+": "+state
            }
            text=lines.isEmpty ? "No cloud projects enabled. Mac files still works separately." : lines.joined(separator:"\n\n")
            if let checked=status["checkedAt"] as? Double { text += "\n\nLast checked: "+Date(timeIntervalSince1970:checked).formatted() }
        }
        NSApp.activate(ignoringOtherApps:true)
        let alert=NSAlert();alert.messageText="Cloud project status";alert.informativeText=text;alert.runModal()
    }
    func showError(_ text:String) {
        let alert=NSAlert();alert.messageText="JaDE connection";alert.informativeText=text;alert.runModal()
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
