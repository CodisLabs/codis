/* ****************************************************************************
 * Start OHLC series code													 *
 *****************************************************************************/

// 1 - Set default options
defaultPlotOptions.ohlc = merge(defaultPlotOptions.column, {
	lineWidth: 1,
	tooltip: {
		pointFormat: '<span style="color:{point.color}">\u25CF</span> <b> {series.name}</b><br/>' + // docs
			'Open: {point.open}<br/>' +
			'High: {point.high}<br/>' +
			'Low: {point.low}<br/>' +
			'Close: {point.close}<br/>'
	},
	states: {
		hover: {
			lineWidth: 3
		}
	},
	threshold: null
	//upColor: undefined
});

// 2 - Create the OHLCSeries object
var OHLCSeries = extendClass(seriesTypes.column, {
	type: 'ohlc',
	pointArrayMap: ['open', 'high', 'low', 'close'], // array point configs are mapped to this
	toYData: function (point) { // return a plain array for speedy calculation
		return [point.open, point.high, point.low, point.close];
	},
	pointValKey: 'high',

	pointAttrToOptions: { // mapping between SVG attributes and the corresponding options
		stroke: 'color',
		'stroke-width': 'lineWidth'
	},
	upColorProp: 'stroke',

	/**
	 * Postprocess mapping between options and SVG attributes
	 */
	getAttribs: function () {
		seriesTypes.column.prototype.getAttribs.apply(this, arguments);
		var series = this,
			options = series.options,
			stateOptions = options.states,
			upColor = options.upColor || series.color,
			seriesDownPointAttr = merge(series.pointAttr),
			upColorProp = series.upColorProp;

		seriesDownPointAttr[''][upColorProp] = upColor;
		seriesDownPointAttr.hover[upColorProp] = stateOptions.hover.upColor || upColor;
		seriesDownPointAttr.select[upColorProp] = stateOptions.select.upColor || upColor;

		each(series.points, function (point) {
			if (point.open < point.close && !point.options.color) {
				point.pointAttr = seriesDownPointAttr;
			}
		});
	},

	/**
	 * Translate data points from raw values x and y to plotX and plotY
	 */
	translate: function () {
		var series = this,
			yAxis = series.yAxis;

		seriesTypes.column.prototype.translate.apply(series);

		// do the translation
		each(series.points, function (point) {
			// the graphics
			if (point.open !== null) {
				point.plotOpen = yAxis.translate(point.open, 0, 1, 0, 1);
			}
			if (point.close !== null) {
				point.plotClose = yAxis.translate(point.close, 0, 1, 0, 1);
			}

		});
	},

	/**
	 * Draw the data points
	 */
	drawPoints: function () {
		var series = this,
			points = series.points,
			chart = series.chart,
			pointAttr,
			plotOpen,
			plotClose,
			crispCorr,
			halfWidth,
			path,
			graphic,
			crispX;


		each(points, function (point) {
			if (point.plotY !== UNDEFINED) {

				graphic = point.graphic;
				pointAttr = point.pointAttr[point.selected ? 'selected' : ''] || series.pointAttr[NORMAL_STATE];

				// crisp vector coordinates
				crispCorr = (pointAttr['stroke-width'] % 2) / 2;
				crispX = mathRound(point.plotX) - crispCorr;  // #2596
				halfWidth = mathRound(point.shapeArgs.width / 2);

				// the vertical stem
				path = [
					'M',
					crispX, mathRound(point.yBottom),
					'L',
					crispX, mathRound(point.plotY)
				];

				// open
				if (point.open !== null) {
					plotOpen = mathRound(point.plotOpen) + crispCorr;
					path.push(
						'M',
						crispX,
						plotOpen,
						'L',
						crispX - halfWidth,
						plotOpen
					);
				}

				// close
				if (point.close !== null) {
					plotClose = mathRound(point.plotClose) + crispCorr;
					path.push(
						'M',
						crispX,
						plotClose,
						'L',
						crispX + halfWidth,
						plotClose
					);
				}

				// create and/or update the graphic
				if (graphic) {
					graphic
						.attr(pointAttr) // #3897
						.animate({ d: path });
				} else {
					point.graphic = chart.renderer.path(path)
						.attr(pointAttr)
						.add(series.group);
				}

			}


		});

	},

	/**
	 * Disable animation
	 */
	animate: null


});
seriesTypes.ohlc = OHLCSeries;
/* ****************************************************************************
 * End OHLC series code													   *
 *****************************************************************************/
