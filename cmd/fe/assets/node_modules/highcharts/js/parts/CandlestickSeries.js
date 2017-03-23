/* ****************************************************************************
 * Start Candlestick series code											  *
 *****************************************************************************/

// 1 - set default options
defaultPlotOptions.candlestick = merge(defaultPlotOptions.column, {
	lineColor: 'black',
	lineWidth: 1,
	states: {
		hover: {
			lineWidth: 2
		}
	},
	tooltip: defaultPlotOptions.ohlc.tooltip,
	threshold: null,
	upColor: 'white'
	// upLineColor: null
});

// 2 - Create the CandlestickSeries object
var CandlestickSeries = extendClass(OHLCSeries, {
	type: 'candlestick',

	/**
	 * One-to-one mapping from options to SVG attributes
	 */
	pointAttrToOptions: { // mapping between SVG attributes and the corresponding options
		fill: 'color',
		stroke: 'lineColor',
		'stroke-width': 'lineWidth'
	},
	upColorProp: 'fill',

	/**
	 * Postprocess mapping between options and SVG attributes
	 */
	getAttribs: function () {
		seriesTypes.ohlc.prototype.getAttribs.apply(this, arguments);
		var series = this,
			options = series.options,
			stateOptions = options.states,
			upLineColor = options.upLineColor || options.lineColor,
			hoverStroke = stateOptions.hover.upLineColor || upLineColor,
			selectStroke = stateOptions.select.upLineColor || upLineColor;

		// Add custom line color for points going up (close > open).
		// Fill is handled by OHLCSeries' getAttribs.
		each(series.points, function (point) {
			if (point.open < point.close) {

				// If an individual line color is set, we need to merge the
				// point attributes, because they are shared between all up
				// points by inheritance from OHCLSeries.
				if (point.lineColor) {
					point.pointAttr = merge(point.pointAttr);
					upLineColor = point.lineColor;
				}

				point.pointAttr[''].stroke = upLineColor;
				point.pointAttr.hover.stroke = hoverStroke;
				point.pointAttr.select.stroke = selectStroke;
			}
		});
	},

	/**
	 * Draw the data points
	 */
	drawPoints: function () {
		var series = this,  //state = series.state,
			points = series.points,
			chart = series.chart,
			pointAttr,
			seriesPointAttr = series.pointAttr[''],
			plotOpen,
			plotClose,
			topBox,
			bottomBox,
			hasTopWhisker,
			hasBottomWhisker,
			crispCorr,
			crispX,
			graphic,
			path,
			halfWidth;


		each(points, function (point) {

			graphic = point.graphic;
			if (point.plotY !== UNDEFINED) {

				pointAttr = point.pointAttr[point.selected ? 'selected' : ''] || seriesPointAttr;

				// crisp vector coordinates
				crispCorr = (pointAttr['stroke-width'] % 2) / 2;
				crispX = mathRound(point.plotX) - crispCorr; // #2596
				plotOpen = point.plotOpen;
				plotClose = point.plotClose;
				topBox = math.min(plotOpen, plotClose);
				bottomBox = math.max(plotOpen, plotClose);
				halfWidth = mathRound(point.shapeArgs.width / 2);
				hasTopWhisker = mathRound(topBox) !== mathRound(point.plotY);
				hasBottomWhisker = bottomBox !== point.yBottom;
				topBox = mathRound(topBox) + crispCorr;
				bottomBox = mathRound(bottomBox) + crispCorr;

				// create the path
				path = [
					'M',
					crispX - halfWidth, bottomBox,
					'L',
					crispX - halfWidth, topBox,
					'L',
					crispX + halfWidth, topBox,
					'L',
					crispX + halfWidth, bottomBox,
					'Z', // Use a close statement to ensure a nice rectangle #2602
					'M',
					crispX, topBox,
					'L',
					crispX, hasTopWhisker ? mathRound(point.plotY) : topBox, // #460, #2094
					'M',
					crispX, bottomBox,
					'L',
					crispX, hasBottomWhisker ? mathRound(point.yBottom) : bottomBox // #460, #2094
				];

				if (graphic) {
					graphic
						.attr(pointAttr) // #3897
						.animate({ d: path });
				} else {
					point.graphic = chart.renderer.path(path)
						.attr(pointAttr)
						.add(series.group)
						.shadow(series.options.shadow);
				}

			}
		});

	}


});

seriesTypes.candlestick = CandlestickSeries;

/* ****************************************************************************
 * End Candlestick series code												*
 *****************************************************************************/
