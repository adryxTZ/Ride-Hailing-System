import type { NextConfig } from "next";

/** The dashboard is a client app over the Platform API (NEXT_PUBLIC_API_URL). */
const config: NextConfig = {
  reactStrictMode: true,
  transpilePackages: ["@platform-web/api-client"],
  output: "standalone", // self-contained server for the Docker image (infra/compose.yml)
};

export default config;
