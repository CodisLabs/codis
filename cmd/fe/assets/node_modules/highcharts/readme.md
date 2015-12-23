#Highcharts JS
Highcharts JS is a JavaScript charting library based on SVG and VML rendering.

* Official website:  [www.highcharts.com](http://www.highcharts.com)
* Official download: [www.highcharts.com/download](http://www.highcharts.com/download)
* Licensing: [www.highcharts.com/license](http://www.highcharts.com/license)
* Support: [www.highcharts.com/support](http://www.highcharts.com/support)

## Reporting issues
We use GitHub Issues as our official bug tracker. We strive to keep this a clean, maintainable and searchable record of our open and closed bugs, therefore we kindly ask you to obey some rules before reporting an issue:

1. Make sure the report is accompanied by a reproducible demo. The ideal demo is created by forking [our standard jsFiddle](http://jsfiddle.net/highcharts/llexl/), adding your own code and stripping it down to an absolute minimum needed to demonstrate the bug.
* Always add information on what browser it applies to, and other information needed for us to debug.
* It may be that the bug is already fixed. Try your chart with our latest work from http://github.highcharts.com/master/highcharts.js before reporting.
* For feature requests, tech support and general discussion, don't use GitHub Issues. See [www.highcharts.com/support](http://www.highcharts.com/support) for the appropriate channels.

## Apply a fix
When an issue is resolved, we commit a fix and mark the issue closed. This doesn't mean that a new release is available with the fix applied, but that it is fixed in the development code and will be added to the next stable release. Stable versions are typically released every 1-3 months. To try out the fix immediately, you can run http://github.highcharts.com/highcharts.js or http://github.highcharts.com/highstock.js from any website, but do not use these URLs in production.

If the fix is critical for your project, we recommend that you apply the fix to the latest stable release of Highcharts or Highstock instead of running the latest file found on GitHub, where other untested changes are also present. Most issues are resolved in single patches that don't conflict with other changes. If you're not into Git and don't want to install and learn that procedure, here's how to apply it quickly with help of online tools:
* Locate your issue on GitHub, for example [#2510](https://github.com/highslide-software/highcharts.com/issues/2510).
* Most issues are closed directly from a commit. Go to that commit, for example [d5e176](https://github.com/highslide-software/highcharts.com/commit/d5e176b5c01bb60402c1f6347993a818e2ab4035).
* Now add `.patch` to the URL to view the [patch file](https://github.com/highslide-software/highcharts.com/commit/d5e176b5c01bb60402c1f6347993a818e2ab4035.patch).
* The patch file will show diffs from all files changed. Here it's important to be aware that `highcharts.src.js`, `highstock.src.js` and `highcharts-more.src.js` are concatenated from parts files. Instead of applying the patches from part files, you only need those from the concatenated files.
* If you need to patch `highcharts.src.js`, copy the diff for that file. Start selecting including the line `diff --git a/js/highcharts.src.js b/js/highcharts.src.js` and select all text until the next diff statement for the next file.
* Now the patch is on your clipboard, open another tab at [i-tools.org/diff](http://i-tools.org/diff).
* Under "Original file", click "By URL" and enter `http://code.highcharts.com/highcharts.src.js` or another source file from the latest stable release, see [code.highcharts.com](http://code.highcharts.com).
* Under "Second file or patch file" click "Direct input" and paste the diff from your clipboard.
* Click the "Patch" button, and if everything is okay you should now have a patched file.
* The next (optional) step is to compile the source code in order to reduce file size. Copy the result from the patched file.
* Go to the [Closure Compiler web app](http://closure-compiler.appspot.com/home).
* Paste the patched file contents to the left and click "Compile".

