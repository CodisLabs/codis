
var UNDEFINED,
	Axis = Highcharts.Axis,
	Chart = Highcharts.Chart,
	Color = Highcharts.Color,
	Point = Highcharts.Point,
	Pointer = Highcharts.Pointer,
	Legend = Highcharts.Legend,
	LegendSymbolMixin = Highcharts.LegendSymbolMixin,
	Renderer = Highcharts.Renderer,
	Series = Highcharts.Series,
	SVGRenderer = Highcharts.SVGRenderer,
	VMLRenderer = Highcharts.VMLRenderer,

	addEvent = Highcharts.addEvent,
	each = Highcharts.each,
	error = Highcharts.error,
	extend = Highcharts.extend,
	extendClass = Highcharts.extendClass,
	merge = Highcharts.merge,
	pick = Highcharts.pick,
	defaultOptions = Highcharts.getOptions(),
	seriesTypes = Highcharts.seriesTypes,
	defaultPlotOptions = defaultOptions.plotOptions,
	wrap = Highcharts.wrap,
	noop = function () {};

	