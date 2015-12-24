/* ****************************************************************************
 * Start PlotBand series code											      *
 *****************************************************************************/
/**
 * This is an experiment of implementing plotBands and plotLines as a series.
 * It could solve problems with export, updating etc., add tooltip and mouse events,
 * and provide a more compact and consistent implementation.
 * Demo: http://jsfiddle.net/highcharts/5Rbf6/
 */

(function (H) {

var seriesTypes = H.seriesTypes,
	merge = H.merge,
	defaultPlotOptions = H.getOptions().plotOptions,
	extendClass = H.extendClass,
	each = H.each,
	Series = H.Series;

// 1 - set default options
defaultPlotOptions.plotband = merge(defaultPlotOptions.column, {
	lineWidth: 0,
	//onXAxis: false,
	threshold: null
});

// 2 - Create the CandlestickSeries object
seriesTypes.plotband = extendClass(seriesTypes.column, {
	type: 'plotband',

	/**
	 * One-to-one mapping from options to SVG attributes
	 */
	pointAttrToOptions: { // mapping between SVG attributes and the corresponding options
		fill: 'color',
		stroke: 'lineColor',
		'stroke-width': 'lineWidth'
	},

	animate: function () {},

	translate: function () {
		var series = this,
			xAxis = series.xAxis,
			yAxis = series.yAxis;

		Series.prototype.translate.apply(series);

		each(series.points, function (point) {
			var onXAxis = point.onXAxis,
				ownAxis = onXAxis ? xAxis : yAxis,
				otherAxis = onXAxis ? yAxis : xAxis,
				from = ownAxis.toPixels(point.from, true),
				to = ownAxis.toPixels(point.to, true),
				start = Math.min(from, to),
				width = Math.abs(to - from);

			point.plotY = 1; // lure ColumnSeries.drawPoints
			point.shapeType = 'rect';
			point.shapeArgs = ownAxis.horiz ? {
				x: start,
				y: 0,
				width: width,
				height: otherAxis.len
			} : {
				x: 0,
				y: start,
				width: otherAxis.len,
				height: width
			};
		});
	},

	/**
	 * Draw the data points
	 */
	_drawPoints: function () {

	}


});

}(Highcharts));

/* ****************************************************************************
 * End PlotBand series code												      *
 *****************************************************************************/
