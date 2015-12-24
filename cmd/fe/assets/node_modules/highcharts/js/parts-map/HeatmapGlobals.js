
var UNDEFINED,
	Axis = Highcharts.Axis,
	Chart = Highcharts.Chart,
	Color = Highcharts.Color,
	Legend = Highcharts.Legend,
	LegendSymbolMixin = Highcharts.LegendSymbolMixin,
	Series = Highcharts.Series,
	Point = Highcharts.Point,

	defaultOptions = Highcharts.getOptions(),
	each = Highcharts.each,
	extend = Highcharts.extend,
	extendClass = Highcharts.extendClass,
	merge = Highcharts.merge,
	pick = Highcharts.pick,
	seriesTypes = Highcharts.seriesTypes,
	wrap = Highcharts.wrap,
	noop = function () {};

	