import js from "@eslint/js";
import globals from "globals";

export default [
  {
    ignores: ["public/**", "docs/**", "dist/**", "node_modules/**"],
  },
  {
    files: ["src/**/*.js"],
    languageOptions: {
      ecmaVersion: 2021,
      sourceType: "module",
      globals: {
        ...globals.browser,
        Timeless: "readonly",
        Fragment: "readonly",
        Button: "readonly",
        Input: "readonly",
        Img: "readonly",
        Link: "readonly",
        Dialog: "readonly",
        DropdownMenu: "readonly",
        DL: "readonly",
        dl$: "readonly",
        DownloadTaskModel: "readonly",
        DownloaderModel: "readonly",
        DLUtils: "readonly",
        VirtualListView: "readonly",
        isElement: "readonly",
        For: "readonly",
        Show: "readonly",
        Switch: "readonly",
        Match: "readonly",
        AspectRatio: "readonly",
        View: "readonly",
        Txt: "readonly",
        SVG: "readonly",
        Circle: "readonly",
        ref: "readonly",
        refarr: "readonly",
        refobj: "readonly",
        computed: "readonly",
        combine: "readonly",
      },
    },
    rules: {
      ...js.configs.recommended.rules,
      "no-undef": "error",
      "no-unused-vars": "warn",
      "no-console": "off",
      "no-redeclare": "off",
    },
  },
];
