// Gulpfile.js

"use strict";

var gulp = require("gulp"),
  eslint = require("gulp-eslint"),
  less = require("gulp-less"),
  minifyCSS = require("gulp-minify-css"),
  path = require("path"),
  notify = require("gulp-notify"),
  clean = require("gulp-clean"),
  rename = require("gulp-rename"),
  uglify = require("gulp-uglify");


gulp.task("less", function() {
  gulp.src("src/less/bootstrap-dialog.less")
    .pipe(gulp.dest("dist/less"))
    .pipe(less({
      paths: [path.join(__dirname, "less", "includes")]
    }))
    .pipe(gulp.dest("dist/css"))
    .pipe(gulp.dest("src/css"))
    .pipe(gulp.dest("examples/assets/bootstrap-dialog/css"))
    .pipe(rename("bootstrap-dialog.min.css"))
    .pipe(minifyCSS())
    .pipe(gulp.dest("dist/css"))
    .pipe(gulp.dest("examples/assets/bootstrap-dialog/css"));
});

gulp.task("lint", function() {
  gulp.src(["src/js/bootstrap-dialog.js"])
    .pipe(eslint())
    .pipe(eslint.format());
});

gulp.task("dist", ["clean", "less"], function() {
  gulp.src(["src/js/bootstrap-dialog.js"])
    .pipe(gulp.dest("dist/js"))
    .pipe(gulp.dest("examples/assets/bootstrap-dialog/js"))
    .pipe(rename("bootstrap-dialog.min.js"))
    .pipe(uglify())
    .pipe(gulp.dest("dist/js"))
    .pipe(gulp.dest("examples/assets/bootstrap-dialog/js"))
    .pipe(notify({
      message: "Build task completed."
    }));
});

gulp.task("clean", function() {
  return gulp.src(["dist/"], {
    read: false
  })
    .pipe(clean());
});

gulp.task("default", ["clean"], function() {
  gulp.start("dist");
});
