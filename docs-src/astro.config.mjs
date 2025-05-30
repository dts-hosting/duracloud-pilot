// @ts-check
import {defineConfig} from "astro/config";

export default defineConfig({
    site: 'https://dts-hosting.github.io',
    base: process.env.ASTRO_BASE || "/",
    build: {
        assets: "_assets"
    },
    outDir: "../docs"
});
