"""The booking calendar the site cannot vouch for: run.sh serves /kontakt
with a feed that never answers and no copy kept. In place of the months,
a sentence and the number to call, with script and without."""

from conftest import BASE
from playwright.sync_api import expect


def check(page):
    section = page.locator("#kalender")
    expect(section).to_contain_text("Kalenderen er ikke tilgjengelig akkurat nå. Ring +47 90 91 98 12 for å spørre om en dato.")
    expect(section.get_by_role("link", name="+47 90 91 98 12")).to_have_attribute("href", "tel:+4790919812")
    assert page.locator("#kalender table, #kalender input, #kalender details, [data-calendar]").count() == 0
    assert page.locator("script[src*='/static/calendar.js']").count() == 0


def test_hidden(desktop):
    check(desktop.page("/kontakt"))


def test_hidden_without_javascript(browser):
    ctx = browser.new_context(java_script_enabled=False)
    ctx.route("https://umami.kradalby.no/**", lambda r: r.abort())
    page = ctx.new_page()
    page.goto(BASE + "/kontakt")
    check(page)
    ctx.close()


def test_no_weekends_in_llms_txt(desktop):
    # A request, not a page: WebKit's text viewer styles inline, which the CSP
    # rightly refuses.
    assert "Ledige helger" not in desktop.ctx.request.get(BASE + "/llms.txt").text()
