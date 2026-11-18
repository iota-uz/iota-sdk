// The type of a scoped CSS module: `styles.board` is the unique class name the build gives `.board`.
declare module "*.module.css" {
  const classes: Readonly<Record<string, string>>
  export default classes
}
