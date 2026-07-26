import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.dirname(fileURLToPath(import.meta.url));
const pluginRoot = path.join(
  root,
  "com.ulanzi.commandexecutor.ulanziPlugin"
);

export default {
  mode: "production",
  target: "node20",
  entry: path.join(pluginRoot, "plugin/app.js"),
  output: {
    path: path.join(pluginRoot, "dist"),
    filename: "app.js",
    library: {
      type: "module"
    },
    chunkFormat: "module"
  },
  experiments: {
    outputModule: true
  },
  optimization: {
    minimize: false
  },
  devtool: false
};
