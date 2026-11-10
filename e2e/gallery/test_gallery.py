"""Browser checks of the gallery: ui.Gallery's grid and the elm/gallery lightbox.

run.sh imports the import tests' miniature old site and serves it at $BASE:
/galleri with its $PHOTOS photos, the 2026 Christmas market with four more,
and /e2e/two, a page with two galleries. Every test runs with the site's Content-Security-Policy and
fails on a violation or a page error.
"""

import os

import pytest
from playwright.sync_api import expect, sync_playwright

BASE = os.environ["BASE"]
OUT = os.environ["OUT"]
PHOTOS = int(os.environ["PHOTOS"])
BROWSERS = os.environ.get("BROWSERS", "chromium firefox webkit").split()

DIALOG = "[role=dialog]"
SHOWN = "[role=dialog] figure img"

# Registered before any page script, so it sees every violation.
CSP_LISTENER = """
window.__csp = [];
document.addEventListener('securitypolicyviolation',
  e => window.__csp.push(e.violatedDirective + ' ' + e.blockedURI));
"""


@pytest.fixture(scope="session")
def pw():
    with sync_playwright() as p:
        yield p


@pytest.fixture(scope="session", params=BROWSERS)
def browser(pw, request):
    b = getattr(pw, request.param).launch()
    yield b
    b.close()


class Checked:
    """A browser context whose pages must end without CSP violations or errors."""

    def __init__(self, browser, **opts):
        self.ctx = browser.new_context(**opts)
        # Analytics is an outside service; tests stay off the network.
        self.ctx.route(
            "https://umami.kradalby.no/**",
            lambda r: r.fulfill(status=200, content_type="text/javascript", body=""),
        )
        self.ctx.add_init_script(CSP_LISTENER)
        self.pages = []
        self.errors = []

    def page(self, path):
        page = self.ctx.new_page()
        page.on("pageerror", lambda e: self.errors.append(str(e)))
        page.on(
            "console",
            lambda m: m.type == "error" and self.errors.append(m.text),
        )
        resp = page.goto(BASE + path)
        assert resp.ok, path
        self.pages.append(page)
        return page

    def close(self):
        for p in self.pages:
            assert p.evaluate("window.__csp") == [], "CSP violation"
        assert self.errors == []
        self.ctx.close()


@pytest.fixture
def desktop(browser):
    c = Checked(browser, viewport={"width": 1280, "height": 800})
    yield c
    c.close()


@pytest.fixture
def phone(browser):
    opts = {"viewport": {"width": 390, "height": 844}, "has_touch": True}
    if browser.browser_type.name != "firefox":  # Firefox has no mobile mode
        opts["is_mobile"] = True
    c = Checked(browser, **opts)
    yield c
    c.close()


def galleries(page):
    return page.evaluate(
        "[...document.querySelectorAll('[data-gallery]')].map(g => JSON.parse(g.dataset.gallery))"
    )


def link(page, index, gallery=0):
    return page.locator("[data-gallery]").nth(gallery).locator(f"a[data-index='{index}']")


def expect_shown(page, photo):
    expect(page.locator(SHOWN)).to_have_attribute("src", photo["src"])


def test_csp_is_enforced_and_listened_to(desktop):
    page = desktop.page("/galleri")
    csp = page.request.get(BASE + "/galleri").headers["content-security-policy"]
    # The harness sets no analytics, so scripts are the site's own only.
    assert "script-src 'self';" in csp and "style-src 'self';" in csp
    # An inline script must be refused, and the listener must see it; then
    # forget it so the fixture's own check covers the gallery alone.
    page.evaluate(
        "document.body.append(Object.assign(document.createElement('script'), {textContent: 'window.x = 1'}))"
    )
    page.wait_for_function("() => window.__csp.length > 0")
    assert page.evaluate("window.x") is None
    page.evaluate("window.__csp = []")
    desktop.errors.clear()  # the browser also logs the refusal


def test_one_script_only_on_gallery_pages(desktop):
    page = desktop.page("/galleri")
    assert page.locator("script[src*='/static/gallery.js']").count() == 1
    page = desktop.page("/lokaler")
    assert page.locator("script[src*='/static/gallery.js']").count() == 0


def test_without_javascript_links_open_the_photo(browser):
    ctx = browser.new_context(java_script_enabled=False)
    ctx.route("https://umami.kradalby.no/**", lambda r: r.abort())
    page = ctx.new_page()
    page.goto(BASE + "/galleri")
    assert page.locator("[data-gallery] a[data-index]").count() == PHOTOS
    href = link(page, 0).get_attribute("href")
    assert "/full-" in href and href.endswith(".jpg")
    with page.expect_navigation() as nav:
        link(page, 0).click()
    assert nav.value.ok
    assert nav.value.headers["content-type"] == "image/jpeg"
    assert page.url == BASE + href
    ctx.close()


def test_keys_browse_and_wrap(desktop):
    page = desktop.page("/galleri")
    photos = galleries(page)[0]
    link(page, 0).click()
    expect(page.locator(DIALOG)).to_have_attribute("aria-label", f"Bilde 1 av {PHOTOS}")
    expect_shown(page, photos[0])
    page.keyboard.press("ArrowRight")
    expect_shown(page, photos[1])
    page.keyboard.press("ArrowLeft")
    expect_shown(page, photos[0])
    page.keyboard.press("ArrowLeft")  # wraps, as the old gallery did
    expect_shown(page, photos[-1])
    expect(page.locator(DIALOG)).to_have_attribute("aria-label", f"Bilde {PHOTOS} av {PHOTOS}")
    page.keyboard.press("ArrowRight")
    expect_shown(page, photos[0])


def test_buttons_browse_and_close(desktop):
    page = desktop.page("/galleri")
    photos = galleries(page)[0]
    link(page, 1).click()
    expect_shown(page, photos[1])
    page.get_by_role("button", name="Neste bilde").click()
    expect_shown(page, photos[2])
    page.get_by_role("button", name="Forrige bilde").click()
    expect_shown(page, photos[1])
    assert page.evaluate("getComputedStyle(document.documentElement).overflow") == "hidden"
    page.get_by_role("button", name="Lukk").click()
    expect(page.locator(DIALOG)).to_have_count(0)
    expect(link(page, 1)).to_be_focused()
    assert page.evaluate("getComputedStyle(document.documentElement).overflow") != "hidden"


def test_shows_alt_text_and_credit(desktop):
    page = desktop.page("/galleri")
    photos = galleries(page)[0]
    i = next(i for i, p in enumerate(photos) if p["alt"] == "Blomster fra @bondenshage")
    link(page, i).click()
    expect(page.locator(SHOWN)).to_have_attribute("alt", "Blomster fra @bondenshage")
    expect(page.locator("[role=dialog] figcaption")).to_contain_text("Blomster fra @bondenshage")
    credit = page.locator("[role=dialog] figcaption a")
    expect(credit).to_have_text("Foto: Lucky Thirteen")
    expect(credit).to_have_attribute("href", "https://luckythirteen.no/")


def test_escape_closes_and_returns_focus(desktop):
    page = desktop.page("/galleri")
    link(page, 3).focus()
    page.keyboard.press("Enter")
    expect(page.get_by_role("button", name="Lukk")).to_be_focused()
    page.keyboard.press("Escape")
    expect(page.locator(DIALOG)).to_have_count(0)
    expect(link(page, 3)).to_be_focused()


def test_backdrop_closes_photo_does_not(desktop):
    page = desktop.page("/galleri")
    link(page, 0).click()
    page.locator(SHOWN).click()
    expect(page.locator(DIALOG)).to_have_count(1)
    page.mouse.click(640, 4)
    expect(page.locator(DIALOG)).to_have_count(0)


def test_focus_stays_in_the_lightbox(desktop):
    page = desktop.page("/galleri")
    link(page, 0).click()
    expect(page.get_by_role("button", name="Lukk")).to_be_focused()
    inside = "document.activeElement.closest('[role=dialog]') !== null"
    for key in ["Tab"] * 8 + ["Shift+Tab"] * 8:
        page.keyboard.press(key)
        assert page.evaluate(inside), key


def test_focus_stays_in_the_lightbox_after_clicking_the_photo(desktop):
    page = desktop.page("/galleri")
    link(page, 0).click()
    page.locator(SHOWN).click()
    inside = "document.activeElement.closest('[role=dialog]') !== null"
    assert page.evaluate(inside), "a click on the photo keeps focus in the dialog"
    page.keyboard.press("Tab")
    expect(page.get_by_role("button", name="Lukk")).to_be_focused()
    page.locator(SHOWN).click()
    page.keyboard.press("Shift+Tab")
    expect(page.get_by_role("button", name="Neste bilde")).to_be_focused()


def test_announces_the_photo_it_moves_to(desktop):
    page = desktop.page("/galleri")
    photos = galleries(page)[0]
    link(page, 0).click()
    live = page.locator("[role=dialog] [aria-live=polite]")
    expect(live).to_have_text(f"Bilde 1 av {PHOTOS}: {photos[0]['alt']}")
    page.get_by_role("button", name="Neste bilde").click()
    expect(live).to_have_text(f"Bilde 2 av {PHOTOS}: {photos[1]['alt']}")
    # The same node, changed: a live region read out, not a new one.
    assert live.evaluate("p => p === document.querySelector('[role=dialog]').firstElementChild")


def test_fits_the_window_it_opens_in(desktop):
    """A phone turned while the lightbox is closed: the next photo fits the new window."""
    page = desktop.page("/galleri")
    link(page, 0).click()
    wide = page.locator(SHOWN).evaluate("i => [i.width, i.height]")
    page.keyboard.press("Escape")
    page.set_viewport_size({"width": 390, "height": 844})
    page.wait_for_function("() => innerWidth === 390")
    link(page, 0).click()
    narrow = page.locator(SHOWN).evaluate("i => [i.width, i.height]")
    assert narrow[0] <= 390 - 16 < wide[0], (wide, narrow)


def test_two_galleries_on_one_page(desktop):
    page = desktop.page("/e2e/two")
    first, second = galleries(page)
    assert page.locator("script[src*='/static/gallery.js']").count() == 1

    link(page, 0, gallery=1).click()
    expect_shown(page, second[0])
    expect(page.locator(DIALOG)).to_have_attribute("aria-label", f"Bilde 1 av {len(second)}")
    for _ in second:  # a full round stays in this gallery
        page.keyboard.press("ArrowRight")
    expect_shown(page, second[0])
    page.keyboard.press("Escape")
    expect(link(page, 0, gallery=1)).to_be_focused()

    link(page, 2, gallery=0).click()
    expect_shown(page, first[2])
    expect(page.locator(DIALOG)).to_have_count(1)


def test_event_page_gallery(desktop):
    """The Christmas market's further photos open in the lightbox too."""
    page = desktop.page("/arrangementer/2026/julemarked")
    assert page.locator("script[src*='/static/gallery.js']").count() == 1
    (photos,) = galleries(page)
    assert len(photos) == 4
    link(page, 3).click()
    expect_shown(page, photos[3])
    expect(page.locator(DIALOG)).to_have_attribute("aria-label", "Bilde 4 av 4")
    page.keyboard.press("ArrowRight")  # wraps within the event's photos
    expect_shown(page, photos[0])
    page.keyboard.press("Escape")
    expect(link(page, 3)).to_be_focused()


def test_lightbox_fetches_the_size_the_viewport_needs(desktop, phone):
    for c, path in ((desktop, "/galleri"), (phone, "/galleri")):
        page = c.page(path)
        link(page, 0).click()
        img = page.locator(SHOWN)
        page.wait_for_function(
            "() => document.querySelector('[role=dialog] figure img')?.complete && document.querySelector('[role=dialog] figure img').currentSrc !== ''"
        )
        shown, src, dpr, srcset = img.evaluate("i => [i.width, i.currentSrc, devicePixelRatio, i.srcset]")
        width = int(src.rsplit("-", 1)[1].split(".")[0])
        # The photo's own rungs: a crop between ladder steps adds its width.
        widths = sorted(int(s.rsplit(" ", 1)[1][:-1]) for s in srcset.split(", "))
        want = next(w for w in widths if w >= shown * dpr)
        assert width == want, (shown, dpr, src)
        assert not src.endswith(".jpg"), src  # AVIF or WebP where supported


def test_lightbox_preloads_neighbours(desktop):
    page = desktop.page("/galleri")
    photos = galleries(page)[0]
    link(page, 0).click()
    for neighbour in (photos[1], photos[-1]):
        sha = neighbour["src"].split("/")[3]
        page.wait_for_function(
            "sha => performance.getEntriesByType('resource').some(e => e.name.includes(sha + '/full-') && e.name.includes('-1200.'))",
            arg=sha,
        )


def test_reduced_motion(browser):
    for motion, want in (("reduce", "0s"), ("no-preference", "0.2s")):
        c = Checked(browser, reduced_motion=motion)
        page = c.page("/galleri")
        link(page, 0).click()
        got = page.locator(DIALOG).evaluate("d => getComputedStyle(d).transitionDuration")
        assert got == want, motion
        c.close()


def test_mounting_moves_nothing(browser):
    def layout(block):
        ctx = browser.new_context(viewport={"width": 1280, "height": 800})
        ctx.route("https://umami.kradalby.no/**", lambda r: r.abort())
        # Boxes come from width/height alone; a decoded variant's rounding
        # would move the page by a pixel either way and hide what this checks.
        ctx.route("**/img/**", lambda r: r.abort())
        if block:
            ctx.route("**/static/gallery.js*", lambda r: r.abort())
        page = ctx.new_page()
        page.goto(BASE + "/galleri", wait_until="load")
        page.evaluate("document.fonts.ready")
        got = page.evaluate(
            """[document.documentElement.scrollHeight,
                ...[...document.querySelectorAll('[data-gallery] a')].slice(0, 8)
                   .map(a => JSON.stringify(a.getBoundingClientRect()))]"""
        )
        mounted = page.evaluate("document.body.lastChild.nodeType")
        ctx.close()
        return got, mounted

    without, _ = layout(block=True)
    with_js, last = layout(block=False)
    assert with_js == without
    assert last == 3  # Elm's closed view: an empty text node, not a box


def swipe(page, browser_name, dx, dy=0):
    x, y = 200, 420
    if browser_name == "chromium":
        # Real touch input, through the browser's touch-action handling.
        cdp = page.context.new_cdp_session(page)
        cdp.send("Input.dispatchTouchEvent", {"type": "touchStart", "touchPoints": [{"x": x, "y": y, "id": 1}]})
        for step in range(1, 6):
            cdp.send(
                "Input.dispatchTouchEvent",
                {"type": "touchMove", "touchPoints": [{"x": x + dx * step / 5, "y": y + dy * step / 5, "id": 1}]},
            )
        cdp.send("Input.dispatchTouchEvent", {"type": "touchEnd", "touchPoints": []})
        cdp.detach()
    else:
        page.locator(DIALOG).evaluate(
            """(d, [x, y, dx, dy]) => {
                 const fire = (type, f) => d.dispatchEvent(new PointerEvent(type, {
                   pointerId: 7, pointerType: 'touch', isPrimary: true, bubbles: true,
                   clientX: x + dx * f, clientY: y + dy * f }));
                 fire('pointerdown', 0); fire('pointermove', 0.5); fire('pointermove', 1); fire('pointerup', 1);
               }""",
            [x, y, dx, dy],
        )


def test_phone_swipe(phone, browser):
    page = phone.page("/galleri")
    photos = galleries(page)[0]
    link(page, 0).tap()
    expect_shown(page, photos[0])
    assert page.locator(SHOWN).evaluate("i => i.getBoundingClientRect().right") <= 390
    # Before any swipe: Chromium drops a tap that follows a touch gesture
    # within a few hundred ms, as a phone does after a fling.
    page.get_by_role("button", name="Neste bilde").tap()
    expect_shown(page, photos[1])

    name = browser.browser_type.name
    swipe(page, name, -150)
    expect_shown(page, photos[2])
    swipe(page, name, 150)
    expect_shown(page, photos[1])
    swipe(page, name, -30)  # a wobble, not a swipe
    swipe(page, name, -60, 120)  # mostly vertical
    expect(page.locator(DIALOG)).to_have_attribute("aria-label", f"Bilde 2 av {PHOTOS}")


@pytest.mark.parametrize("width,height", [(390, 844), (1280, 800)])
def test_screenshots(browser, width, height):
    # Reduced motion: the lightbox's fade would catch it half drawn.
    c = Checked(browser, viewport={"width": width, "height": height}, reduced_motion="reduce")
    page = c.page("/galleri")
    name = f"{OUT}/{browser.browser_type.name}-{width}"
    decoded = "imgs => Promise.all(imgs.map(i => i.decode()))"
    page.evaluate(f"() => ({decoded})([...document.querySelectorAll('[data-gallery] img')].slice(0, 2))")
    shoot(c, page, f"{name}-grid.png")
    link(page, 0).click()
    page.locator(SHOWN).evaluate(f"i => ({decoded})([i])")
    shoot(c, page, f"{name}-lightbox.png")
    c.close()


def shoot(c, page, path):
    # Playwright's WebKit screenshot injects a <style>, which the CSP rightly
    # refuses; check the page before it and forget the screenshot's own.
    assert page.evaluate("window.__csp") == [] and c.errors == []
    page.screenshot(path=path, caret="initial")
    page.evaluate("window.__csp = []")
    c.errors[:] = [e for e in c.errors if "Content Security Policy" not in e]


def test_data_matches_the_grid(desktop):
    """The single source: every link's index finds its own photo in the data."""
    page = desktop.page("/galleri")
    photos = galleries(page)[0]
    pairs = page.evaluate(
        """[...document.querySelectorAll('[data-gallery] a[data-index]')]
             .map(a => [Number(a.dataset.index), a.querySelector('img').getAttribute('src')])"""
    )
    assert [i for i, _ in pairs] == list(range(len(photos)))
    assert [src for _, src in pairs] == [p["src"] for p in photos]
