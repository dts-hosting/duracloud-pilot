// @ts-check
import { defineConfig } from "astro/config";
import starlight from "@astrojs/starlight";

export default defineConfig({
  site: "https://dts-hosting.github.io",
  base: process.env.ASTRO_BASE || "/",
  build: {
    assets: "_assets",
  },
  integrations: [
    starlight({
      title: "DuraCloud Docs",
      editLink: {
        baseUrl:
          "https://github.com/dts-hosting/duracloud-pilot/edit/main/docs-src/",
      },
      favicon: "/favicon.ico",
      logo: {
        src: "./src/assets/duracloud-icon.png",
      },
      sidebar: [
        { slug: "index", label: "Introduction" },
        {
          label: "About",
          collapsed: false,
          autogenerate: { directory: "about" },
        },
        {
          label: "User Manual",
          collapsed: false,
          autogenerate: { directory: "guides/user/" },
        },
        {
          label: "Tech Docs",
          collapsed: false,
          autogenerate: { directory: "guides/technical/" },
        },
      ],
      social: [
        {
          icon: "github",
          label: "GitHub",
          href: "https://github.com/dts-hosting/duracloud-pilot",
        },
      ],
    }),
  ],
  outDir: "../docs",
});
