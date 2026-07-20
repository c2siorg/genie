/** @type {import('next').NextConfig} */
const nextConfig = {
  // Static export → plain HTML/CSS/JS that the Go binary embeds and serves.
  output: 'export',
  // Genie serves the UI under /ui/, so assets must resolve there.
  basePath: '/ui',
  assetPrefix: '/ui',
  // Emit index.html per route (directory-style) so the Go static handler can serve it.
  trailingSlash: true,
  // No image optimization server in a static export.
  images: { unoptimized: true },
  // Export lands here; a later step copies it into pkg/web/handlers/ui/.
  distDir: '.next',
};

export default nextConfig;
