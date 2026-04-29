#!/usr/bin/env python3
import argparse
from pathlib import Path

from playwright.sync_api import sync_playwright


VIEWS = ["overview", "violations", "quarantine", "replay", "config"]


def main() -> None:
    parser = argparse.ArgumentParser(description="Capture WriteFence local UI screenshots.")
    parser.add_argument("--url", default="http://127.0.0.1:9622/_writefence")
    parser.add_argument("--out", default="docs/assets/ui")
    args = parser.parse_args()

    out_dir = Path(args.out)
    out_dir.mkdir(parents=True, exist_ok=True)

    with sync_playwright() as p:
        browser = p.chromium.launch()
        page = browser.new_page(viewport={"width": 1440, "height": 1000}, device_scale_factor=1)
        page.goto(args.url, wait_until="networkidle")
        page.screenshot(path=out_dir / "writefence-ui-overview.png", full_page=True)

        for view in VIEWS[1:]:
            page.get_by_role("button", name=view.capitalize()).click()
            page.wait_for_timeout(250)
            page.screenshot(path=out_dir / f"writefence-ui-{view}.png", full_page=True)

        browser.close()

    for view in VIEWS:
        print(out_dir / f"writefence-ui-{view}.png")


if __name__ == "__main__":
    main()
