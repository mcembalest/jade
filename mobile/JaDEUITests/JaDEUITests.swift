import XCTest

final class JaDEUITests: XCTestCase {
    func testSanjanaUpdatesWithoutChat() throws {
        let app=XCUIApplication()
        app.launchEnvironment["JADE_OFFLINE_UI_TEST"]="1"
        app.launchEnvironment["JADE_UI_TEST_ID"]=UUID().uuidString
        app.launchEnvironment["JADE_SANJANA_FIXTURE"]="""
        {"enabled":true,"paused":false,"messages":[{"id":"desktop","role":"assistant","text":"A little company, wherever you are.","proactive":true}],"pending":[{"text":"A discovery saved in Cloudflare","sources":[{"title":"Read the original","url":"https://example.com"}]}]}
        """
        app.launch();app.tabBars.buttons["Sanjana"].tap()
        XCTAssertTrue(app.staticTexts["Sanjana’s corner"].waitForExistence(timeout:5))
        XCTAssertTrue(app.staticTexts["A little company, wherever you are."].exists)
        let shot=XCTAttachment(screenshot:app.screenshot());shot.name="Sanjana on iPhone";shot.lifetime = .keepAlways;add(shot)
        app.swipeUp()
        XCTAssertFalse(app.descendants(matching:.any).matching(identifier:"Message Sanjana").firstMatch.exists)
        XCTAssertFalse(app.buttons["Send to Sanjana"].exists)
        XCTAssertFalse(app.buttons["Look for discoveries"].exists)
        XCTAssertTrue(app.staticTexts["Updates from Sanjana"].exists)
        app.terminate();app.launch();app.tabBars.buttons["Sanjana"].tap()
        XCTAssertTrue(app.staticTexts["A little company, wherever you are."].waitForExistence(timeout:5))
        app.tabBars.buttons["Notes"].tap();XCTAssertTrue(app.navigationBars["JaDE"].exists)
        app.tabBars.buttons["Mac files"].tap();XCTAssertTrue(app.buttons["Cloud projects · available with Mac off"].exists)
    }
    @MainActor func testBulkDownloadAndConflictResolution() async throws {
        #if LOCAL_CLOUD_TEST
        let project="ui-"+UUID().uuidString
        func post(_ suffix:String,_ body:[String:Any]) async throws {
            var request=URLRequest(url:URL(string:"http://127.0.0.1:8799/v1/projects/"+project+suffix)!)
            request.httpMethod="POST";request.httpBody=try JSONSerialization.data(withJSONObject:body)
            request.setValue("Bearer agent-test-secret",forHTTPHeaderField:"Authorization")
            request.setValue("application/json",forHTTPHeaderField:"Content-Type")
            let (_,response)=try await URLSession.shared.data(for:request)
            XCTAssertEqual((response as? HTTPURLResponse)?.statusCode,200)
        }
        try await post("",["name":"Offline UI Test","enabled":true])
        try await post("/file",["path":"code.py","content":"original code","baseRevision":"","mutationId":"initial"])
        try await post("/file",["path":"second.py","content":"offline second file","baseRevision":"","mutationId":"second"])
        let app=XCUIApplication()
        app.launchEnvironment["JADE_OFFLINE_UI_TEST"]="1"
        app.launchEnvironment["JADE_UI_TEST_ID"]=UUID().uuidString
        app.launchEnvironment["JADE_CLOUD_LOCAL_UI_TEST"]="1"
        func openProject() {
            app.tabBars.buttons["Mac files"].tap()
            app.buttons["Cloud projects · available with Mac off"].tap()
            XCTAssertTrue(app.staticTexts["Offline UI Test"].waitForExistence(timeout:20))
            app.staticTexts["Offline UI Test"].tap()
        }
        app.launch();openProject()
        app.buttons["Download missing files for offline use"].tap()
        XCTAssertTrue(app.staticTexts["2/2 files saved on this iPhone"].waitForExistence(timeout:20))
        app.staticTexts["code.py"].tap()
        let editor=app.textViews["Cloud project text"]
        XCTAssertTrue(editor.waitForExistence(timeout:10));editor.tap();editor.typeText("phone draft ")
        try await post("/file",["path":"code.py","content":"cloud changed","baseRevision":"initial","mutationId":"changed"])
        app.buttons["Submit to Cloudflare"].tap()
        XCTAssertTrue(app.buttons["Resolve conflict…"].waitForExistence(timeout:20));app.buttons["Resolve conflict…"].tap()
        XCTAssertTrue(app.buttons["Use cloud version"].waitForExistence(timeout:5));app.buttons["Use cloud version"].tap()
        let merged=app.textViews["Conflict resolution text"]
        merged.tap();merged.typeText("combined ")
        let result=merged.value as? String
        app.swipeUp()
        app.buttons["Save resolution and preserve original"].tap()
        XCTAssertTrue(editor.waitForExistence(timeout:5));XCTAssertEqual(editor.value as? String,result)
        app.terminate();app.launch();openProject()
        XCTAssertTrue(app.staticTexts["2/2 files saved on this iPhone"].exists)
        app.staticTexts["code.py"].tap();XCTAssertTrue(editor.waitForExistence(timeout:10));XCTAssertEqual(editor.value as? String,result)
        let shot=XCTAttachment(screenshot:app.screenshot());shot.name="Offline download and preserved conflict resolution";shot.lifetime = .keepAlways;add(shot)
        #else
        throw XCTSkip("Requires isolated local Cloudflare test server")
        #endif
    }
    func testCloudProjectWithMacAgentStopped() throws {
        #if LIVE_CLOUD_TEST
        let app=XCUIApplication()
        app.launchEnvironment["JADE_OFFLINE_UI_TEST"]="1"
        app.launchEnvironment["JADE_UI_TEST_ID"]=UUID().uuidString
        app.launch()
        app.tabBars.buttons["Mac files"].tap()
        app.buttons["Cloud projects · available with Mac off"].tap()
        XCTAssertTrue(app.staticTexts["JaDE Cloud Test"].waitForExistence(timeout:30))
        app.staticTexts["JaDE Cloud Test"].tap()
        XCTAssertTrue(app.staticTexts["wedding-test.py"].waitForExistence(timeout:20))
        app.staticTexts["wedding-test.py"].tap()
        let editor=app.textViews["Cloud project text"]
        XCTAssertTrue(editor.waitForExistence(timeout:20))
        editor.tap();editor.typeText("# phone while Mac off\n")
        let draft=editor.value as? String
        XCTAssertTrue(app.staticTexts["Draft saved on iPhone · not submitted"].exists)
        app.terminate();app.launch()
        app.tabBars.buttons["Mac files"].tap()
        app.buttons["Cloud projects · available with Mac off"].tap()
        XCTAssertTrue(app.staticTexts["JaDE Cloud Test"].waitForExistence(timeout:30))
        app.staticTexts["JaDE Cloud Test"].tap()
        app.staticTexts["wedding-test.py"].tap()
        XCTAssertTrue(editor.waitForExistence(timeout:20));XCTAssertEqual(editor.value as? String,draft)
        app.buttons["Submit to Cloudflare"].tap()
        XCTAssertTrue(app.staticTexts["Stored in Cloudflare · Mac pending"].waitForExistence(timeout:30))
        let shot=XCTAttachment(screenshot:app.screenshot());shot.name="Cloud accepted while Mac agent stopped";shot.lifetime = .keepAlways;add(shot)
        #else
        throw XCTSkip("Explicit live cloud-project test only")
        #endif
    }
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
