// Entry point. index.html loads this file with ?v=<version>; everything else is
// imported by plain relative URLs, so shared modules exist exactly once.
import { boot } from "./shell.js";

boot();
