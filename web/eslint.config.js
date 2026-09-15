import js from "@eslint/js";
import prettier from "eslint-config-prettier";
import reactHooks from "eslint-plugin-react-hooks";
import reactRefresh from "eslint-plugin-react-refresh";
import globals from "globals";
import tseslint from "typescript-eslint";

export default tseslint.config(
  {
    ignores: ["dist"],
  },
  js.configs.recommended,
  ...tseslint.configs.recommended,
  {
    files: ["**/*.{ts,tsx}"],
    languageOptions: {
      ecmaVersion: 2022,
      globals: globals.browser,
    },
    plugins: {
      "react-hooks": reactHooks,
      "react-refresh": reactRefresh,
    },
    rules: {
      ...reactHooks.configs.recommended.rules,
      "react-refresh/only-export-components": ["warn", { allowConstantExport: true }],
    },
  },
  {
    // Every HTTP call goes through src/api/client.ts: it attaches X-Token and turns the error
    // envelope into ApiError, and the rest of the app relies on both.
    files: ["src/**/*.{ts,tsx}"],
    ignores: ["src/api/client.ts", "src/**/*.test.{ts,tsx}"],
    rules: {
      "no-restricted-globals": [
        "error",
        {
          name: "fetch",
          message:
            "Add a typed function to src/api/client.ts and call that; it sends X-Token and throws ApiError.",
        },
      ],
      "no-restricted-properties": [
        "error",
        {
          object: "window",
          property: "fetch",
          message: "Add a typed function to src/api/client.ts and call that.",
        },
        {
          object: "globalThis",
          property: "fetch",
          message: "Add a typed function to src/api/client.ts and call that.",
        },
      ],
    },
  },
  prettier,
);
