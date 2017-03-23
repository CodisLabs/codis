/**
 * This script allows debugging by including the raw parts files without using a server backend
 */

var files = [
	"Globals.js",
	"Utilities.js",
	"PathAnimation.js",
	"JQueryAdapter.js",
	"Adapters.js",
	"Options.js",
	"Color.js",
	"SvgRenderer.js",
	"VmlRenderer.js",
	"CanVGRenderer.js",
	"Tick.js",
	"PlotLineOrBand.js",
	"StackItem.js",
	"Axis.js",
	"Tooltip.js",
	"Pointer.js",
	"Legend.js",
	"Chart.js",
	"Series.js",
	"LineSeries.js",
	"AreaSeries.js",
	"SplineSeries.js",
	"AreaSplineSeries.js",
	"ColumnSeries.js",
	"BarSeries.js",
	"ScatterSeries.js",
	"PieSeries.js",
	"Facade.js"
];

// Parse the path from the script tag
var scripts = document.getElementsByTagName('script'),
	path;

for (var i = 0; i < scripts.length; i++) {
	if (scripts[i].src.indexOf('highcharts.debug.js') !== -1) {
		path = scripts[i].src.replace(/highcharts\.debug\.js\?(.*?)$/, '') + 'parts/'
	}
}

console.log('--- Running individual parts ---')
// Include the individual files
for (var i = 0; i < files.length; i++) {
	document.write('<script src="' + path + files[i] + '?' + (new Date()).getTime() +'"></script>')
}
