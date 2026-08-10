import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { NavigationIcon } from "./NavigationIcon";
import { routeIds } from "./navigation";

describe("NavigationIcon", () => {
  it("renders a consistent SVG icon for every route", () => {
    for (const route of routeIds) {
      const markup = renderToStaticMarkup(
        createElement(NavigationIcon, { route }),
      );

      expect(markup).toContain('<svg aria-hidden="true"');
      expect(markup).toContain('viewBox="0 0 24 24"');
      expect(markup).toMatch(/<(path|rect|circle)/);
    }
  });
});
