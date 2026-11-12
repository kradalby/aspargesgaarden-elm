"""Browser checks of the booking calendar on /kontakt: ui.BookingCalendar's
months and weekend list, and the elm/calendar app that takes them over.

run.sh serves calendar.ics as the feed, with the server's clock on the
browser tests' fixed day, Tuesday 10 November 2026, so the answers never
change with the date the suite runs on.
"""

import datetime
import re

import pytest
from playwright.sync_api import expect

from conftest import BASE, OUT, Checked, shoot

# The taken days of calendar.ics.
TAKEN = {
    datetime.date(2026, 11, 13),
    datetime.date(2026, 11, 14),
    datetime.date(2026, 11, 15),
    datetime.date(2026, 11, 21),
    datetime.date(2027, 5, 29),
    datetime.date(2027, 6, 5),
    datetime.date(2027, 6, 12),
    datetime.date(2027, 6, 19),
    datetime.date(2027, 6, 26),
    datetime.date(2028, 9, 2),
}

MONTHS = "januar februar mars april mai juni juli august september oktober november desember".split()


def parse(text):
    """'Lørdag 15. mai 2027' to a date."""
    _, day, month, year = text.split()
    return datetime.date(int(year), MONTHS.index(month) + 1, int(day.rstrip(".")))


def captions(page):
    return page.locator("#kalender caption").all_inner_texts()


def ask(page, iso):
    page.locator("#kalender-dato").fill(iso)
    page.get_by_role("button", name="Sjekk").click()


def answer(page):
    return page.locator("#kalender-svar")


def test_without_javascript(browser):
    ctx = browser.new_context(java_script_enabled=False)
    ctx.route("https://umami.kradalby.no/**", lambda r: r.abort())
    page = ctx.new_page()
    page.goto(BASE + "/kontakt")
    assert captions(page) == ["November 2026", "Desember 2026", "Januar 2027"]
    assert page.locator("#kalender td.bg-rust").count() == 4  # 13–15 and 21 November
    assert page.locator("#kalender input").count() == 0  # no question it cannot answer
    assert "Vil du spørre om en dato? Ring +47 90 91 98 12." in page.locator("#kalender").inner_text()
    # The weekends as text, in the HTML whether or not the list is open.
    text = page.locator("#kalender details").text_content()
    assert "13.–15. november" in text and "Opptatt" in text and "Ledig lørdag og søndag" not in text
    assert "Kalenderen går til september 2028." in page.locator("#kalender").inner_text()
    assert page.locator("iframe").count() == 0
    ctx.close()


def test_scripts(desktop):
    page = desktop.page("/kontakt")
    assert page.locator("script[src*='/static/calendar.js']").count() == 1
    assert page.locator("script[src*='/static/gallery.js']").count() == 0
    page = desktop.page("/lokaler")
    assert page.locator("script[src*='/static/calendar.js']").count() == 0


@pytest.mark.parametrize("width,height", [(390, 844), (1280, 800)])
def test_booting_moves_nothing(browser, width, height):
    def layout(block):
        ctx = browser.new_context(viewport={"width": width, "height": height})
        ctx.route("https://umami.kradalby.no/**", lambda r: r.abort())
        if block:
            ctx.route("**/static/calendar.js*", lambda r: r.abort())
        page = ctx.new_page()
        page.goto(BASE + "/kontakt", wait_until="load")
        page.evaluate("document.fonts.ready")
        if not block:
            expect(page.get_by_role("button", name="Neste")).to_be_visible()
        got = page.evaluate(
            """[...document.querySelectorAll('#kalender ul:first-of-type li, #kalender caption, #kalender td, #kalender th')]
                 .map(e => [e.tagName, e.textContent, JSON.stringify(e.getBoundingClientRect())])"""
        )
        ctx.close()
        return got

    # The question fills the box the page left for it: nothing under it moves.
    assert layout(block=False) == layout(block=True)


def test_paging(desktop):
    page = desktop.page("/kontakt")
    previous = page.get_by_role("button", name="Forrige")
    following = page.get_by_role("button", name="Neste")
    expect(previous).to_have_attribute("aria-disabled", "true")

    following.focus()
    following.press("Enter")
    following.press("Enter")
    expect(page.locator("#kalender caption")).to_have_text(["Januar 2027", "Februar 2027", "Mars 2027"])
    expect(page.locator("#kalender p[aria-live=polite]")).to_have_text("Januar – mars 2027")
    # The focus stays on the button: one key after another pages on.
    assert page.evaluate("document.activeElement.getAttribute('aria-label')") == "Neste måned"

    previous.click()
    expect(page.locator("#kalender caption").first).to_have_text("Desember 2026")

    # Up to the last month the data covers, then no further.
    # (forced: Playwright waits for an aria-disabled button to come back)
    for _ in range(30):
        following.click(force=True)
    expect(page.locator("#kalender caption").last).to_have_text("September 2028")
    expect(following).to_have_attribute("aria-disabled", "true")


def test_taken_date_suggests_free_days_of_its_weekday(desktop):
    page = desktop.page("/kontakt")
    ask(page, "2027-06-12")
    expect(answer(page)).to_contain_text("Lørdag 12. juni 2027 er opptatt.")
    expect(page.locator("#kalender caption")).to_have_text(["Juni 2027", "Juli 2027", "August 2027"])

    buttons = answer(page).get_by_role("button")
    expect(buttons).to_have_count(4)
    days = [parse(t) for t in buttons.all_inner_texts()]
    assert days == sorted(days)
    for d in days:
        assert d.weekday() == 5 and d not in TAKEN, d
    assert days == [datetime.date(2027, 5, 15), datetime.date(2027, 5, 22), datetime.date(2027, 7, 3), datetime.date(2027, 7, 10)]

    # A suggestion moves the window to its month and marks the day.
    buttons.nth(3).click()
    expect(buttons.nth(3)).to_have_attribute("aria-pressed", "true")
    expect(page.locator("#kalender caption").first).to_have_text("Juli 2027")
    marked = page.locator("#kalender td", has=page.locator("text=din dato"))
    expect(marked).to_have_count(1)
    expect(marked).to_contain_text("10")
    assert page.evaluate("document.activeElement.textContent") == "Lørdag 10. juli 2027"


def test_free_and_far_dates(desktop):
    page = desktop.page("/kontakt")
    ask(page, "2027-07-03")
    expect(answer(page)).to_contain_text("Lørdag 3. juli 2027 er ledig.")
    expect(answer(page).get_by_role("link", name="+47 90 91 98 12")).to_be_visible()
    expect(page.locator("#kalender caption").first).to_have_text("Juli 2027")

    ask(page, "2029-06-02")
    expect(answer(page)).to_contain_text("er lenger frem enn kalenderen går.")

    ask(page, "2026-11-09")
    expect(answer(page)).to_contain_text("Mandag 9. november 2026 har vært.")


@pytest.mark.parametrize("width,height", [(390, 844), (1280, 800)])
def test_screenshots(browser, width, height):
    c = Checked(browser, viewport={"width": width, "height": height})
    page = c.page("/kontakt")
    name = f"{OUT}/calendar-{browser.browser_type.name}-{width}"
    shoot(c, page, f"{name}.png")
    ask(page, "2027-06-12")
    expect(answer(page)).to_contain_text("opptatt")
    shoot(c, page, f"{name}-taken.png")
    c.close()


def oslo(*args):
    return datetime.datetime(*args, tzinfo=datetime.timezone(datetime.timedelta(hours=1)))


def test_the_open_tab(desktop):
    """The browser's clock comes in every minute and when the tab is shown
    again. A day before the server's is ignored, a later one moves today on;
    past the server's limit since the page loaded, the tab stops showing
    bookings at all."""
    page = desktop.ctx.new_page()
    desktop.pages.append(page)
    page.clock.install(time=oslo(2026, 11, 10, 23, 50))
    page.goto(BASE + "/kontakt")
    today = page.locator("#kalender td[aria-current=date]")
    expect(today).to_have_text(re.compile(r"^10"))

    def shown_again(at):
        page.clock.set_system_time(at)
        page.evaluate("document.dispatchEvent(new Event('visibilitychange'))")

    shown_again(oslo(2026, 11, 1, 12))
    expect(today).to_have_text(re.compile(r"^10"))
    shown_again(oslo(2026, 11, 11, 0, 5))
    expect(today).to_have_text(re.compile(r"^11"))

    # Past 30 minutes since loading, by the minute's tick alone.
    page.clock.fast_forward("27:00")
    line = page.locator("#kalender").get_by_text("Kalenderen er ikke tilgjengelig akkurat nå.")
    expect(line).to_be_visible()
    expect(line.get_by_role("link", name="+47 90 91 98 12")).to_be_visible()
    assert page.locator("#kalender table, #kalender input, #kalender details").count() == 0


def test_answer_in_view_on_a_phone(phone):
    """The answer grows the box above the months; the browser must not keep
    the months still and scroll the answer away."""
    page = phone.page("/kontakt")
    page.locator("#kalender-dato").scroll_into_view_if_needed()
    ask(page, "2027-06-12")
    expect(answer(page)).to_contain_text("er opptatt.")
    page.wait_for_timeout(300)
    top, bottom = answer(page).locator("p").first.evaluate("e => [e.getBoundingClientRect().top, e.getBoundingClientRect().bottom]")
    assert 0 <= top and bottom <= 844, (top, bottom)
    expect(page.locator("#kalender caption").first).to_have_text("Juni 2027")


@pytest.mark.parametrize("data", ["not json", "{&#34;today&#34;:&#34;someday&#34;}"])
def test_unreadable_data_leaves_the_page_as_it_is(desktop, data):
    """The app never takes the server's months away to show less."""

    def corrupt(route):
        body = route.fetch().text()
        route.fulfill(body=re.sub(r'data-calendar="[^"]*"', f'data-calendar="{data}"', body), content_type="text/html")

    desktop.ctx.route(BASE + "/kontakt", corrupt)
    page = desktop.page("/kontakt")
    page.wait_for_load_state("load")
    assert captions(page) == ["November 2026", "Desember 2026", "Januar 2027"]
    assert page.locator("[data-booted]").count() == 0
    assert page.locator("#kalender-dato").count() == 0
    assert "Vil du spørre om en dato? Ring" in page.locator("#kalender").inner_text()
