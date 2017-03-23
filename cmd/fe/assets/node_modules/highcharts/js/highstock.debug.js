/**
 * This script allows debugging by including the raw parts files without using a server backend
 */
var files = [
	"Globals.js",
	"Utilities.js",
	"PathAnimation.js",
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
	"DataGrouping.js",
    "OHLCSeries.js",
    "CandlestickSeries.js",
    "FlagsSeries.js",
    "Scroller.js",
    "RangeSelector.js",
    "StockNavigation.js",
    "StockChart.js",
    "OrdinalAxis.js",
	"Facade.js"
];

// Parse the path from the script tag
var $tag = $('script[src$="highstock.debug.js"]'),
	path = $tag.attr('src').replace('highstock.debug.js', '') + 'parts/';

// Include the individual files
$.each(files, function (i, file) {
	document.write('<script src="' + path + file + '"></script>')
});
