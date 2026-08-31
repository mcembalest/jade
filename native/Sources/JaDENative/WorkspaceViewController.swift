import AppKit
import GhosttyTerminal
import WebKit

final class WorkspaceViewController: NSViewController, WKScriptMessageHandler {
    private let root: URL
    private let baseURL: URL
    private let splitView = NSSplitView()
    private let terminalContainer = NSView()
    private var webView: WKWebView!
    private var terminalView: TerminalView?
    private var terminalController: TerminalController?
    private var ptySession: PTYSession?
    private var terminalDirectory: URL?

    init(root: URL, baseURL: URL) {
        self.root = root.resolvingSymlinksInPath().standardizedFileURL
        self.baseURL = baseURL
        super.init(nibName: nil, bundle: nil)
    }

    required init?(coder: NSCoder) {
        fatalError("init(coder:) has not been implemented")
    }

    override func loadView() {
        let contentController = WKUserContentController()
        contentController.add(self, name: "jade")
        let configuration = WKWebViewConfiguration()
        configuration.userContentController = contentController
        configuration.websiteDataStore = .default()
        webView = WKWebView(frame: .zero, configuration: configuration)

        splitView.isVertical = false
        splitView.dividerStyle = .thin
        splitView.autosaveName = "JaDEEditorTerminalSplit"
        splitView.addArrangedSubview(webView)
        terminalContainer.wantsLayer = true
        terminalContainer.layer?.backgroundColor = NSColor.black.cgColor
        terminalContainer.isHidden = true
        splitView.addArrangedSubview(terminalContainer)
        view = splitView
    }

    override func viewDidLoad() {
        super.viewDidLoad()
        webView.load(URLRequest(url: baseURL))
    }

    override func viewDidAppear() {
        super.viewDidAppear()
        view.window?.title = root.lastPathComponent + " — JaDE"
        view.window?.makeFirstResponder(webView)
    }

    override func viewDidLayout() {
        super.viewDidLayout()
        terminalView?.fitToSize()
    }

    deinit {
        webView?.configuration.userContentController.removeScriptMessageHandler(forName: "jade")
        ptySession?.stop()
    }

    func userContentController(_ userContentController: WKUserContentController, didReceive message: WKScriptMessage) {
        guard message.name == "jade",
              let payload = message.body as? [String: Any],
              payload["type"] as? String == "terminal"
        else { return }
        let jadePath = payload["jade"] as? String ?? "."
        openTerminal(jadePath: jadePath)
    }

    private func openTerminal(jadePath: String) {
        guard let directory = safeDirectory(jadePath: jadePath) else {
            presentError(NSError(domain: "JaDE", code: 3, userInfo: [NSLocalizedDescriptionKey: "The terminal directory leaves the workspace."]))
            return
        }
        if terminalDirectory != directory {
            do {
                try replaceTerminal(directory: directory)
            } catch {
                presentError(error)
                return
            }
        }
        terminalContainer.isHidden = false
        splitView.setPosition(max(280, splitView.bounds.height * 0.62), ofDividerAt: 0)
        terminalView?.fitToSize()
        if let terminalView {
            view.window?.makeFirstResponder(terminalView)
        }
    }

    private func replaceTerminal(directory: URL) throws {
        ptySession?.stop()
        terminalView?.removeFromSuperview()

        let pty = try PTYSession(directory: directory.path)
        let controller = TerminalController { builder in
            builder.withBackgroundOpacity(1)
            builder.withCustom("font-family", "Berkeley Mono, SFMono-Regular, Menlo")
            builder.withCustom("font-size", "13")
            builder.withCustom("window-padding-x", "8")
            builder.withCustom("window-padding-y", "8")
        }
        let terminal = TerminalView(frame: terminalContainer.bounds)
        terminal.delegate = self
        terminal.configuration = TerminalSurfaceOptions(backend: .inMemory(pty.terminalSession))
        terminal.controller = controller
        terminal.translatesAutoresizingMaskIntoConstraints = false
        terminalContainer.addSubview(terminal)
        NSLayoutConstraint.activate([
            terminal.topAnchor.constraint(equalTo: terminalContainer.topAnchor),
            terminal.leadingAnchor.constraint(equalTo: terminalContainer.leadingAnchor),
            terminal.trailingAnchor.constraint(equalTo: terminalContainer.trailingAnchor),
            terminal.bottomAnchor.constraint(equalTo: terminalContainer.bottomAnchor),
        ])
        terminalDirectory = directory
        ptySession = pty
        terminalController = controller
        terminalView = terminal
    }

    private func safeDirectory(jadePath: String) -> URL? {
        let relative = jadePath == "." ? "" : jadePath
        let candidate = root.appendingPathComponent(relative, isDirectory: true)
            .resolvingSymlinksInPath().standardizedFileURL
        let rootPath = root.path.hasSuffix("/") ? root.path : root.path + "/"
        guard candidate == root || candidate.path.hasPrefix(rootPath) else { return nil }
        var isDirectory: ObjCBool = false
        guard FileManager.default.fileExists(atPath: candidate.path, isDirectory: &isDirectory), isDirectory.boolValue else { return nil }
        return candidate
    }
}

extension WorkspaceViewController: TerminalSurfaceTitleDelegate, TerminalSurfaceCloseDelegate {
    func terminalDidChangeTitle(_ title: String) {
        guard !title.isEmpty else { return }
        view.window?.title = title + " — JaDE"
    }

    func terminalDidClose(processAlive _: Bool) {
        terminalContainer.isHidden = true
        terminalDirectory = nil
        terminalView = nil
        terminalController = nil
        ptySession = nil
        view.window?.makeFirstResponder(webView)
    }
}
