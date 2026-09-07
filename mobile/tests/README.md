# Mobile source editor tests

Run from the repository root. Pure Swift checks cover line selections, Unicode, CRLF, empty files, and mixed indentation:

```sh
mkdir -p .tmp
swiftc mobile/JaDE/SourceTextEditor.swift mobile/tests/SourceTextEditorTests.swift -o .tmp/source-editor-tests
.tmp/source-editor-tests
```

`SourceEditorNative.xcodeproj` is a standalone XCTest bundle with no JaDE app, models, pairing, or network access. It verifies actual UITextView indentation/outdent, native undo/redo, draft-binding updates, and disabled editing. It does not verify the complete phone screen or physical keyboard interaction.

Use a **new dedicated simulator**, not the paired JaDE simulator. Select an installed iOS runtime/device type with `xcrun simctl list runtimes` and `xcrun simctl list devicetypes`, then:

```sh
xcrun simctl create 'JaDE isolated editor tests' DEVICE_TYPE_ID RUNTIME_ID
xcodebuild -project mobile/tests/SourceEditorNative.xcodeproj -scheme NativeTests -destination 'platform=iOS Simulator,id=NEW_SIMULATOR_ID' -derivedDataPath .tmp/source-native-build test
xcrun simctl shutdown NEW_SIMULATOR_ID
xcrun simctl delete NEW_SIMULATOR_ID
```

The app build remains the unsigned command documented in the [mobile README](../README.md). These checks do not require reinstalling the phone app or restarting the Mac connection.
