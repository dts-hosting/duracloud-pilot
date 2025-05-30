// @ts-check
import {defineConfig} from "astro/config";
import starlight from "@astrojs/starlight";

export default defineConfig({
    site: 'https://dts-hosting.github.io',
    base: process.env.ASTRO_BASE || "/",
    build: {
        assets: "_assets"
    },
    integrations: [
        starlight({
            title: "DuraCloud Docs"
        }),
    ],
    outDir: "../docs"
});
