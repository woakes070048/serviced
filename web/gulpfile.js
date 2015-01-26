/* globals require: true */

var gulp = require("gulp"),
    concat = require("gulp-concat"),
    uglify = require("gulp-uglify"),
    rename = require("gulp-rename"),
    jshint = require("gulp-jshint"),
    sequence = require("run-sequence"),
    to5 = require("gulp-6to5");

var paths = {
    // TODO - organize by feature, not type
    src: "static/js/",
    controllers: "static/js/controllers/",
    modules: "static/js/modules/",
    build: "static/js/",
    thirdpartySrc: "static/thirdparty/",
    thirdpartyBuild: "static/thirdparty/"
};

var to5Config = {
    format: {
        parentheses: true,
        comments: true,
        compact: false,
        indent: {
            adjustMultilineComment: false,
            style: "    ",
            base: 0
        }
    }
};

// files to be concatenated/minified to make
// controlplane.js
var controlplaneFiles = [
    paths.src + "main.js",
    paths.modules + "*.js",
    paths.controllers + "*.js"
];

// Third-party library files to be concatenated/minified to make thirdparty.js
var thirdpartyFiles = [
    paths.thirdpartySrc + "jquery/jquery.js",
    paths.thirdpartySrc + "jquery-timeago/jquery.timeago.js",
    paths.thirdpartySrc + "jquery-ui/ui/jquery-ui.js",
    paths.thirdpartySrc + "jquery-datetimepicker/jquery.datetimepicker.js",

    paths.thirdpartySrc + "bootstrap/dist/js/bootstrap.js",
    paths.thirdpartySrc + "bootstrap/js/tooltip.js",
    paths.thirdpartySrc + "bootstrap/js/popover.js",

    paths.thirdpartySrc + "elastic/elasticsearch.js",

    paths.thirdpartySrc + "angular/angular.js",
    paths.thirdpartySrc + "angular/angular-route.js",
    paths.thirdpartySrc + "angular/angular-cookies.js",
    paths.thirdpartySrc + "angular-dragdrop/angular-dragdrop.js",
    paths.thirdpartySrc + "angular-translate/angular-translate.js",
    paths.thirdpartySrc + "angular-translate/angular-translate-loader-static-files/angular-translate-loader-static-files.js",
    paths.thirdpartySrc + "angular-translate/angular-translate-loader-url/angular-translate-loader-url.js",
    paths.thirdpartySrc + "angular-cache/angular-cache.js",
    // FIXME: Can't fnid a matching version of angular-moment.min.js.
    //      WHAT VERSION DO WE REALLY HAVE?
    paths.thirdpartySrc + "angular-moment/angular-moment.min.js",
    paths.thirdpartySrc + "angular-sticky/sticky.js",

    paths.thirdpartySrc + "d3/d3.js",
    paths.thirdpartySrc + "graphlib/graphlib.js",
    paths.thirdpartySrc + "dagre-d3/dagre-d3.js",

    paths.thirdpartySrc + "codemirror/lib/codemirror.js",
    paths.thirdpartySrc + "codemirror/mode/properties/properties.js",
    paths.thirdpartySrc + "codemirror/mode/yaml/yaml.js",
    paths.thirdpartySrc + "codemirror/mode/xml/xml.js",
    paths.thirdpartySrc + "codemirror/mode/shell/shell.js",
    paths.thirdpartySrc + "codemirror/mode/javascript/javascript.js",
    paths.thirdpartySrc + "angular-ui-codemirror/ui-codemirror.js",
];

gulp.task("default", ["concat"]);
gulp.task("release", function(){
    // last arg is a callback function in case
    // of an error.
    sequence("concat3rdparty", "uglify3rdparty", function(){});
    sequence("lint", "concat", "uglify", function(){});
});

gulp.task("concat", function(){
    return gulp.src(controlplaneFiles)
        .pipe(to5(to5Config))
        .pipe(concat("controlplane.js"))
        .pipe(gulp.dest(paths.build));
});

gulp.task("uglify", function(){
    return gulp.src(paths.build + "controlplane.js")
        .pipe(uglify())
        .pipe(rename(paths.build + "controlplane.min.js"))
        .pipe(gulp.dest("./"));
});

gulp.task("concat3rdparty", function(){
    return gulp.src(thirdpartyFiles)
        .pipe(concat("thirdparty.js"))
        .pipe(gulp.dest(paths.thirdpartyBuild));
});

gulp.task("uglify3rdparty", function(){
    return gulp.src(paths.thirdpartyBuild + "thirdparty.js")
        .pipe(uglify())
        .pipe(rename(paths.thirdpartyBuild + "thirdparty.min.js"))
        .pipe(gulp.dest("./"));
});

gulp.task("watch", function(){
    gulp.watch(paths.controllers + "/*", ["concat"]);
    gulp.watch(paths.modules + "/*", ["concat"]);
    gulp.watch(paths.src + "/main.js", ["concat"]);
});

gulp.task("test", function(){
    // TODO - unit tests
    // TODO - functional tests
});

gulp.task("lint", function(){
    return gulp.src(controlplaneFiles)
        .pipe(jshint(".jshintrc"))
        .pipe(jshint.reporter("jshint-stylish"))
        .pipe(jshint.reporter("fail"));
});

