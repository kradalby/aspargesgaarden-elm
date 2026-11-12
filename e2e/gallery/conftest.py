"""What the browser suites share: the server under test, one browser per
engine, and page contexts that fail on a CSP violation or a page error."""

import os

import pytest
from playwright.sync_api import sync_playwright

BASE = os.environ["BASE"]
OUT = os.environ["OUT"]
BROWSERS = os.environ.get("BROWSERS", "chromium firefox webkit").split()

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


def shoot(c, page, path):
    # Playwright's WebKit screenshot injects a <style>, which the CSP rightly
    # refuses; check the page before it and forget the screenshot's own.
    assert page.evaluate("window.__csp") == [] and c.errors == []
    page.screenshot(path=path, caret="initial")
    page.evaluate("window.__csp = []")
    c.errors[:] = [e for e in c.errors if "Content Security Policy" not in e]
