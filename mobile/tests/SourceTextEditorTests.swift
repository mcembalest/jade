import Foundation

@main struct SourceTextEditorTests {
    static func main() {
        func check(_ text: String, _ selection: NSRange, _ outdent: Bool, _ expected: String, _ expectedSelection: NSRange) {
            guard let edit = SourceIndentEdit.make(text: text, selection: selection, outdent: outdent) else {
                fatalError("Expected edit for \(text.debugDescription)")
            }
            let actual = (text as NSString).replacingCharacters(in: edit.range, with: edit.replacement)
            precondition(actual == expected, "Wrong contents: \(actual.debugDescription)")
            precondition(edit.selection == expectedSelection, "Wrong selection: \(edit.selection), expected \(expectedSelection)")
        }
        check("", NSRange(location: 0, length: 0), false, "    ", NSRange(location: 4, length: 0))
        check("let x = 1", NSRange(location: 4, length: 0), false, "    let x = 1", NSRange(location: 8, length: 0))
        check("one\ntwo\nthree", NSRange(location: 0, length: 8), false, "    one\n    two\nthree", NSRange(location: 4, length: 12))
        check("one\ntwo", NSRange(location: 0, length: 5), false, "    one\n    two", NSRange(location: 4, length: 9))
        check("    one\n  two\n\tthree", NSRange(location: 0, length: 20), true, "one\ntwo\nthree", NSRange(location: 0, length: 13))
        check("  one", NSRange(location: 1, length: 0), true, "one", NSRange(location: 0, length: 0))
        check("😀a\n  β", NSRange(location: 5, length: 2), true, "😀a\nβ", NSRange(location: 4, length: 1))
        check("one\r\ntwo\r\n", NSRange(location: 0, length: 10), false, "    one\r\n    two\r\n", NSRange(location: 4, length: 14))
        check("one\n", NSRange(location: 4, length: 0), false, "one\n    ", NSRange(location: 8, length: 0))
        precondition(SourceIndentEdit.make(text: "one", selection: NSRange(location: 0, length: 0), outdent: true) == nil)
        precondition(SourceIndentEdit.make(text: "one", selection: NSRange(location: 5, length: 0), outdent: false) == nil)
        print("Source editor: 11 selection, indentation, Unicode and line-ending checks passed")
    }
}
