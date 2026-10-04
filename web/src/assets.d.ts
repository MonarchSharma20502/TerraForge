// Type declarations for the assets the bundler inlines.

declare module "*.css";
declare module "*.svg" {
  const content: string;
  export default content;
}
declare module "*.wasm" {
  const content: string;
  export default content;
}
