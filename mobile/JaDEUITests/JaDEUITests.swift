import XCTest

final class JaDEUITests: XCTestCase {
    func testRemoteCodeEditAndDraftRecovery() throws {
        #if LIVE_REMOTE_TEST
        let app = XCUIApplication()
        app.launchEnvironment["JADE_OFFLINE_UI_TEST"] = "1"
        app.launchEnvironment["JADE_UI_TEST_ID"] = UUID().uuidString
        app.launch()
        app.tabBars.buttons["Mac files"].tap()
        XCTAssertTrue(app.staticTexts["JaDE Remote Test"].waitForExistence(timeout:40))
        app.staticTexts["JaDE Remote Test"].tap()
        XCTAssertTrue(app.staticTexts["sample.py"].waitForExistence(timeout:30))
        app.staticTexts["sample.py"].tap()
        XCTAssertTrue(app.staticTexts["Loaded from Mac"].waitForExistence(timeout:30))
        let editor = app.textViews["Mac file text"]
        editor.tap(); editor.typeText("# edited on iPhone\n")
        let draft = editor.value as? String
        XCTAssertTrue(app.staticTexts["Draft saved on iPhone · not sent to Mac"].exists)
        app.terminate(); app.launch()
        app.tabBars.buttons["Mac files"].tap()
        XCTAssertTrue(app.staticTexts["JaDE Remote Test"].waitForExistence(timeout:40))
        app.staticTexts["JaDE Remote Test"].tap()
        XCTAssertTrue(app.staticTexts["sample.py"].waitForExistence(timeout:30))
        app.staticTexts["sample.py"].tap()
        XCTAssertTrue(editor.waitForExistence(timeout:10))
        XCTAssertEqual(editor.value as? String,draft)
        app.buttons["Save to Mac"].tap()
        XCTAssertTrue(app.staticTexts["Saved on Mac"].waitForExistence(timeout:40))
        let shot = XCTAttachment(screenshot:app.screenshot()); shot.name="Remote code saved on Mac"; shot.lifetime = .keepAlways; add(shot)
        #else
        throw XCTSkip("Explicit remote bridge test only")
        #endif
    }
    // Run explicitly after sending the private jade://pair link to the test
    // simulator. This exercises Apple's URL handoff and the deployed service.
    func testPairedWorkspaceReceivesMacNote() throws {
        #if LIVE_SYNC_TEST
        let system = XCUIApplication(bundleIdentifier: "com.apple.springboard")
        if system.buttons["Open"].waitForExistence(timeout: 5) { system.buttons["Open"].tap() }
        let app = XCUIApplication(); app.activate()
        XCTAssertTrue(app.staticTexts["Your personal workspace"].waitForExistence(timeout: 30))
        XCTAssertTrue(app.staticTexts["Welcome.md"].waitForExistence(timeout: 30))
        app.staticTexts["Welcome.md"].firstMatch.tap()
        XCTAssertTrue(app.staticTexts["Synced with Mac"].waitForExistence(timeout: 30))
        let shot = XCTAttachment(screenshot: app.screenshot()); shot.name="Cloudflare delivery acknowledged by Mac"; shot.lifetime = .keepAlways; add(shot)
        #else
        throw XCTSkip("Explicit live pairing test only")
        #endif
    }
    func testOfflineNoteSurvivesTermination() throws {
        let app = XCUIApplication()
        app.launchEnvironment["JADE_OFFLINE_UI_TEST"] = "1"
        app.launchEnvironment["JADE_UI_TEST_ID"] = UUID().uuidString
        app.launch()
        let title = "Offline proof " + String(UUID().uuidString.prefix(6)) + ".md"
        let note = "An offline edit survives closing and reopening JaDE."
        app.buttons["New note"].tap()
        let name = app.alerts["New note"].textFields.firstMatch
        XCTAssertTrue(name.waitForExistence(timeout: 5))
        name.tap(); name.typeText(title)
        app.alerts["New note"].buttons["Create"].tap()
        app.staticTexts[title].firstMatch.tap()
        let editor = app.textViews["Note text"]
        XCTAssertTrue(editor.waitForExistence(timeout: 5))
        editor.tap(); editor.typeText(note)
        XCTAssertTrue(app.staticTexts["Saved on iPhone · pending sync"].waitForExistence(timeout: 5))
        let shot = XCTAttachment(screenshot: app.screenshot()); shot.name = "Offline note safely staged"; shot.lifetime = .keepAlways; add(shot)
        app.terminate(); app.launch()
        app.staticTexts[title].firstMatch.tap()
        XCTAssertTrue(editor.waitForExistence(timeout: 5))
        XCTAssertEqual(editor.value as? String, note)
        XCTAssertTrue(app.staticTexts["Saved on iPhone · pending sync"].exists)
    }
}
