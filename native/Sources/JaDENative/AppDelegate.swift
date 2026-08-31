import AppKit

@MainActor
final class AppDelegate: NSObject, NSApplicationDelegate {
    private let engine = EngineProcess()
    private var statusItem: NSStatusItem?
    private var currentRoot: URL?
    private var windowController: NSWindowController?
    private var reopenItem: NSMenuItem?
    private var pendingURL: URL?
    private var finishedLaunching = false

    func applicationDidFinishLaunching(_ notification: Notification) {
        configureMenuBar()
        finishedLaunching = true
        let arguments = Array(ProcessInfo.processInfo.arguments.dropFirst())
        if let first = arguments.first, !first.hasPrefix("-") {
            openWorkspace(URL(fileURLWithPath: first))
        } else if let pending = pendingURL {
            pendingURL = nil
            openWorkspace(pending)
        }
    }

    func application(_ application: NSApplication, open urls: [URL]) {
        guard let url = urls.first else { return }
        guard finishedLaunching else {
            pendingURL = url
            return
        }
        openWorkspace(url)
    }

    func applicationWillTerminate(_ notification: Notification) {
        engine.stop()
    }

    private func configureMenuBar() {
        let item = NSStatusBar.system.statusItem(withLength: NSStatusItem.variableLength)
        statusItem = item
        item.button?.title = "◆"
        item.button?.toolTip = "JaDE"

        let menu = NSMenu()
        let openItem = NSMenuItem(title: "Open folder…", action: #selector(chooseFolder), keyEquivalent: "o")
        openItem.target = self
        menu.addItem(openItem)
        let reopen = NSMenuItem(title: "Open current workspace", action: #selector(reopenWorkspace), keyEquivalent: "")
        reopen.target = self
        reopen.isEnabled = false
        reopenItem = reopen
        menu.addItem(reopen)
        menu.addItem(.separator())
        let quitItem = NSMenuItem(title: "Quit JaDE", action: #selector(quit), keyEquivalent: "q")
        quitItem.target = self
        menu.addItem(quitItem)
        item.menu = menu
    }

    @objc private func chooseFolder() {
        let panel = NSOpenPanel()
        panel.title = "Open a repository or folder in JaDE"
        panel.canChooseDirectories = true
        panel.canChooseFiles = false
        panel.allowsMultipleSelection = false
        panel.canCreateDirectories = true
        if panel.runModal() == .OK, let url = panel.url {
            openWorkspace(url)
        }
    }

    @objc private func reopenWorkspace() {
        guard let currentRoot else { return }
        if windowController == nil {
            openWorkspace(currentRoot)
        } else {
            windowController?.showWindow(nil)
            NSApp.activate(ignoringOtherApps: true)
        }
    }

    @objc private func quit() {
        NSApp.terminate(nil)
    }

    private func openWorkspace(_ selected: URL) {
        let root = workspaceRoot(selected)
        currentRoot = root
        reopenItem?.isEnabled = true
        statusItem?.button?.toolTip = "JaDE — \(root.lastPathComponent)"
        windowController?.close()
        windowController = nil
        engine.start(root: root) { [weak self] result in
            guard let self else { return }
            switch result {
            case let .success(baseURL):
                self.presentWorkspace(root: root, baseURL: baseURL)
            case let .failure(error):
                NSApp.presentError(error)
            }
        }
    }

    private func presentWorkspace(root: URL, baseURL: URL) {
        let content = WorkspaceViewController(root: root, baseURL: baseURL)
        let window = NSWindow(contentViewController: content)
        window.title = root.lastPathComponent + " — JaDE"
        window.setContentSize(NSSize(width: 1240, height: 820))
        window.minSize = NSSize(width: 760, height: 520)
        window.styleMask = [.titled, .closable, .miniaturizable, .resizable, .fullSizeContentView]
        window.titlebarAppearsTransparent = true
        window.isReleasedWhenClosed = false
        window.center()
        let controller = NSWindowController(window: window)
        windowController = controller
        controller.showWindow(nil)
        NSApp.activate(ignoringOtherApps: true)
    }

    private func workspaceRoot(_ selected: URL) -> URL {
        var isDirectory: ObjCBool = false
        if FileManager.default.fileExists(atPath: selected.path, isDirectory: &isDirectory), !isDirectory.boolValue {
            return selected.deletingLastPathComponent().resolvingSymlinksInPath().standardizedFileURL
        }
        return selected.resolvingSymlinksInPath().standardizedFileURL
    }
}
