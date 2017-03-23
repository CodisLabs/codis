/**
 * This script allows debugging by including the raw parts files without using a server backend
 */

var files = [
	"Globals.js",
    "Pane.js",
    "RadialAxis.js",
    "AreaRangeSeries.js",
    "AreaSplineRangeSeries.js",
    "ColumnRangeSeries.js",
	"GaugeSeries.js",
	"BoxPlotSeries.js",
	"BubbleSeries.js",
	"Polar.js"
];

// Parse the path from the script tag
var $tag = $('script[src$="highcharts-more.debug.js"]'),
	path = $tag.attr('src').replace('highcharts-more.debug.js', '') + 'parts-more/';

// Include the individual files
$.each(files, function (i, file) {
	document.write('<script src="' + path + file + '"></script>')
});
