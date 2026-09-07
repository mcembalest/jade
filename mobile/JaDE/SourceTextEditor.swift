import Foundation

/// A single replacement, in UIKit's UTF-16 coordinates. No file or delivery knowledge.
struct SourceIndentEdit {
    let range: NSRange
    let replacement: String
    let selection: NSRange

    static func make(text: String, selection: NSRange, outdent: Bool) -> SourceIndentEdit? {
        let source = text as NSString
        guard selection.location <= source.length,
              selection.length <= source.length - selection.location else { return nil }
        // A selection ending at the next line's start does not include that line.
        let lines = source.lineRange(for: selection)
        let block = source.substring(with: lines) as NSString
        var replacements: [(NSRange, String)] = []
        var offset = 0
        repeat {
            let line = block.lineRange(for: NSRange(location: offset, length: 0))
            var remove = 0
            if outdent {
                if offset < block.length && block.character(at: offset) == 9 { remove = 1 }
                else {
                    while remove < 4 && offset + remove < block.length && block.character(at: offset + remove) == 32 { remove += 1 }
                }
            }
            if !outdent || remove > 0 {
                replacements.append((NSRange(location: lines.location + offset, length: remove), outdent ? "" : "    "))
            }
            offset = NSMaxRange(line)
        } while offset < block.length
        guard !replacements.isEmpty else { return nil }
        let updated = NSMutableString(string: block)
        var start = selection.location
        var end = NSMaxRange(selection)
        for (range, value) in replacements.reversed() {
            updated.replaceCharacters(in: NSRange(location: range.location - lines.location, length: range.length), with: value)
            func mapped(_ position: Int) -> Int {
                if position < range.location { return position }
                if position <= NSMaxRange(range) { return range.location + (value as NSString).length }
                return position + (value as NSString).length - range.length
            }
            start = mapped(start); end = mapped(end)
        }
        return SourceIndentEdit(range: lines, replacement: updated as String,
                                selection: NSRange(location: start, length: end - start))
    }
}

#if canImport(UIKit)
import SwiftUI
import UIKit

/// Native editing conveniences; every edit goes through the caller's existing draft binding.
struct SourceTextEditor: UIViewRepresentable {
    @Binding var text: String
    @Environment(\.isEnabled) private var isEnabled

    func makeCoordinator() -> Coordinator { Coordinator(self) }

    func makeUIView(context: Context) -> UITextView {
        let view = UITextView()
        view.delegate = context.coordinator
        view.font = UIFontMetrics(forTextStyle: .body).scaledFont(for: .monospacedSystemFont(ofSize: 17, weight: .regular))
        view.adjustsFontForContentSizeCategory = true
        view.backgroundColor = .systemBackground
        view.textColor = .label
        view.autocapitalizationType = .none
        view.autocorrectionType = .no
        view.spellCheckingType = .no
        view.smartQuotesType = .no
        view.smartDashesType = .no
        view.smartInsertDeleteType = .no
        if #available(iOS 18.0, *) { view.writingToolsBehavior = .none }
        view.keyboardDismissMode = .interactive
        view.textContainerInset = UIEdgeInsets(top: 8, left: 8, bottom: 8, right: 8)
        view.accessibilityLabel = "Mac file text"
        context.coordinator.attach(view)
        return view
    }

    func updateUIView(_ view: UITextView, context: Context) {
        context.coordinator.parent = self
        view.isEditable = isEnabled
        if view.text != text {
            let previous = view.selectedRange
            view.text = text
            let start = min(previous.location, (text as NSString).length)
            view.selectedRange = NSRange(location: start, length: min(previous.length, (text as NSString).length - start))
            // Loading a different remote version must not be undoable into stale contents.
            view.undoManager?.removeAllActions()
        }
        context.coordinator.updateActions()
    }

    @MainActor final class Coordinator: NSObject, UITextViewDelegate {
        var parent: SourceTextEditor
        weak var view: UITextView?
        private var undoButton: UIBarButtonItem!
        private var redoButton: UIBarButtonItem!
        private var editingButtons: [UIBarButtonItem] = []
        init(_ parent: SourceTextEditor) { self.parent = parent }

        func attach(_ view: UITextView) {
            self.view = view
            func button(_ title: String, _ icon: String, _ action: Selector) -> UIBarButtonItem {
                let item = UIBarButtonItem(image: UIImage(systemName: icon), style: .plain, target: self, action: action)
                item.accessibilityLabel = title
                return item
            }
            undoButton = button("Undo", "arrow.uturn.backward", #selector(undo))
            redoButton = button("Redo", "arrow.uturn.forward", #selector(redo))
            let indent = button("Indent four spaces", "increase.indent", #selector(indent))
            let outdent = button("Outdent up to four spaces", "decrease.indent", #selector(outdent))
            let symbols = UIBarButtonItem(title: "{ }", menu: UIMenu(title: "Insert symbol", children:
                ["{", "}", "(", ")", "[", "]", "\"", "'", "`", ":", ";", "=", "/", "\\"].map { symbol in
                    UIAction(title: symbol) { [weak self] _ in self?.insert(symbol) }
                }))
            symbols.accessibilityLabel = "Insert code symbol"
            editingButtons = [indent, outdent, symbols]
            let done = button("Hide keyboard", "keyboard.chevron.compact.down", #selector(dismissKeyboard))
            let toolbar = UIToolbar()
            toolbar.items = [undoButton, redoButton, .flexibleSpace(), outdent, indent, symbols, .flexibleSpace(), done]
            toolbar.sizeToFit()
            view.inputAccessoryView = toolbar
        }

        func updateActions() {
            guard let view else { return }
            let enabled = view.isEditable && view.markedTextRange == nil
            undoButton.isEnabled = enabled && (view.undoManager?.canUndo ?? false)
            redoButton.isEnabled = enabled && (view.undoManager?.canRedo ?? false)
            editingButtons.forEach { $0.isEnabled = enabled }
        }

        func textViewDidChange(_ textView: UITextView) {
            if parent.text != textView.text { parent.text = textView.text }
            updateActions()
        }
        func textViewDidChangeSelection(_ textView: UITextView) { updateActions() }
        @objc private func undo() { view?.undoManager?.undo(); if let view { textViewDidChange(view) } }
        @objc private func redo() { view?.undoManager?.redo(); if let view { textViewDidChange(view) } }
        @objc private func dismissKeyboard() { view?.resignFirstResponder() }
        @objc private func indent() { changeIndent(outdent: false) }
        @objc private func outdent() { changeIndent(outdent: true) }

        private func insert(_ symbol: String) {
            guard let view, view.isEditable, view.markedTextRange == nil else { return }
            view.insertText(symbol)
            textViewDidChange(view)
        }
        private func changeIndent(outdent: Bool) {
            guard let view, view.isEditable, view.markedTextRange == nil,
                  let edit = SourceIndentEdit.make(text: view.text, selection: view.selectedRange, outdent: outdent),
                  let start = view.position(from: view.beginningOfDocument, offset: edit.range.location),
                  let end = view.position(from: start, offset: edit.range.length),
                  let range = view.textRange(from: start, to: end) else { return }
            view.replace(range, withText: edit.replacement)
            view.selectedRange = edit.selection
            view.scrollRangeToVisible(edit.selection)
            textViewDidChange(view)
        }
    }
}
#endif
