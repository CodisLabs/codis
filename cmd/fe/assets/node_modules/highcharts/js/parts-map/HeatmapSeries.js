
/**
 * Extend the default options with map options
 */
defaultOptions.plotOptions.heatmap = merge(defaultOptions.plotOptions.scatter, {
	animation: false,
	borderWidth: 0,
	nullColor: '#F8F8F8',
	dataLabels: {
		formatter: function () { // #2945
			return this.point.value;
		},
		inside: true,
		verticalAlign: 'middle',
		crop: false,
		overflow: false,
		padding: 0 // #3837
	},
	marker: null,
	pointRange: null, // dynamically set to colsize by default
	tooltip: {
		pointFormat: '{point.x}, {point.y}: {point.value}<br/>'
	},
	states: {
		normal: {
			animation: true
		},
		hover: {
			halo: false,  // #3406, halo is not required on heatmaps
			brightness: 0.2
		}
	}
});

// The Heatmap series type
seriesTypes.heatmap = extendClass(seriesTypes.scatter, merge(colorSeriesMixin, {
	type: 'heatmap',
	pointArrayMap: ['y', 'value'],
	hasPointSpecificOptions: true,
	pointClass: extendClass(Point, colorPointMixin),
	supportsDrilldown: true,
	getExtremesFromAll: true,
	directTouch: true,

	/**
	 * Override the init method to add point ranges on both axes.
	 */
	init: function () {
		var options;
		seriesTypes.scatter.prototype.init.apply(this, arguments);

		options = this.options;
		options.pointRange = pick(options.pointRange, options.colsize || 1); // #3758, prevent resetting in setData
		this.yAxis.axisPointRange = options.rowsize || 1; // general point range
	},
	translate: function () {
		var series = this,
			options = series.options,
			xAxis = series.xAxis,
			yAxis = series.yAxis,
			between = function (x, a, b) {
				return Math.min(Math.max(a, x), b);
			};

		series.generatePoints();

		each(series.points, function (point) {
			var xPad = (options.colsize || 1) / 2,
				yPad = (options.rowsize || 1) / 2,
				x1 = between(Math.round(xAxis.len - xAxis.translate(point.x - xPad, 0, 1, 0, 1)), 0, xAxis.len),
				x2 = between(Math.round(xAxis.len - xAxis.translate(point.x + xPad, 0, 1, 0, 1)), 0, xAxis.len),
				y1 = between(Math.round(yAxis.translate(point.y - yPad, 0, 1, 0, 1)), 0, yAxis.len),
				y2 = between(Math.round(yAxis.translate(point.y + yPad, 0, 1, 0, 1)), 0, yAxis.len);

			// Set plotX and plotY for use in K-D-Tree and more
			point.plotX = point.clientX = (x1 + x2) / 2;
			point.plotY = (y1 + y2) / 2;

			point.shapeType = 'rect';
			point.shapeArgs = {
				x: Math.min(x1, x2),
				y: Math.min(y1, y2),
				width: Math.abs(x2 - x1),
				height: Math.abs(y2 - y1)
			};
		});

		series.translateColors();

		// Make sure colors are updated on colorAxis update (#2893)
		if (this.chart.hasRendered) {
			each(series.points, function (point) {
				point.shapeArgs.fill = point.options.color || point.color; // #3311
			});
		}
	},
	drawPoints: seriesTypes.column.prototype.drawPoints,
	animate: noop,
	getBox: noop,
	drawLegendSymbol: LegendSymbolMixin.drawRectangle,

	getExtremes: function () {
		// Get the extremes from the value data
		Series.prototype.getExtremes.call(this, this.valueData);
		this.valueMin = this.dataMin;
		this.valueMax = this.dataMax;

		// Get the extremes from the y data
		Series.prototype.getExtremes.call(this);
	}

}));

