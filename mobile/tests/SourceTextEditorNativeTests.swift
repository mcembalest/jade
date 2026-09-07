import XCTest
import SwiftUI
import UIKit

final class NativeTests: XCTestCase {
    @MainActor func testIndentUndoRedoUsesDraftBinding() {
        var draft = "one\ntwo"
        var saves = 0
        let editor = SourceTextEditor(text: Binding(get: { draft }, set: { draft = $0; saves += 1 }))
        let coordinator = editor.makeCoordinator()
        let view = UITextView()
        view.text = draft
        view.delegate = coordinator
        coordinator.attach(view)
        let window = UIWindow(frame: CGRect(x: 0, y: 0, width: 390, height: 844))
        let controller = UIViewController()
        controller.view = view
        window.rootViewController = controller
        window.makeKeyAndVisible()
        view.becomeFirstResponder()
        view.selectedRange = NSRange(location: 0, length: 5)
        let toolbar = view.inputAccessoryView as! UIToolbar
        let indent = toolbar.items!.first { $0.accessibilityLabel == "Indent four spaces" }!
        _ = coordinator.perform(indent.action)
        XCTAssertEqual(view.text, "    one\n    two")
        XCTAssertEqual(draft, view.text)
        XCTAssertTrue(view.undoManager?.canUndo == true)
        let undo = toolbar.items!.first { $0.accessibilityLabel == "Undo" }!
        _ = coordinator.perform(undo.action)
        XCTAssertEqual(view.text, "one\ntwo")
        XCTAssertEqual(draft, view.text)
        let redo = toolbar.items!.first { $0.accessibilityLabel == "Redo" }!
        _ = coordinator.perform(redo.action)
        XCTAssertEqual(view.text, "    one\n    two")
        XCTAssertEqual(draft, view.text)
        XCTAssertEqual(saves, 3)
        // Outdent is also one native undo step, even with mixed whitespace.
        view.text = "    one\n  two\n\tthree"
        draft = view.text
        view.undoManager?.removeAllActions()
        view.selectedRange = NSRange(location: 0, length: (view.text as NSString).length)
        let outdent = toolbar.items!.first { $0.accessibilityLabel == "Outdent up to four spaces" }!
        _ = coordinator.perform(outdent.action)
        XCTAssertEqual(draft, "one\ntwo\nthree")
        _ = coordinator.perform(undo.action)
        XCTAssertEqual(draft, "    one\n  two\n\tthree")
        view.isEditable = false
        coordinator.updateActions()
        XCTAssertFalse(indent.isEnabled)
        XCTAssertFalse(outdent.isEnabled)
        XCTAssertFalse(undo.isEnabled)
        let before = draft
        _ = coordinator.perform(indent.action)
        XCTAssertEqual(draft, before)
        view.resignFirstResponder()
        window.isHidden = true
    }
}
