/* ****************************************************************************
 * Start Waterfall series code                                                *
 *****************************************************************************/

// 1 - set default options
defaultPlotOptions.waterfall = merge(defaultPlotOptions.column, {
	lineWidth: 1,
	lineColor: '#333',
	dashStyle: 'dot',
	borderColor: '#333',
	dataLabels: {
		inside: true
	},
	states: {
		hover: {
			lineWidthPlus: 0 // #3126
		}
	}
});


// 2 - Create the series object
seriesTypes.waterfall = extendClass(seriesTypes.column, {
	type: 'waterfall',

	upColorProp: 'fill',

	pointValKey: 'y',

	/**
	 * Translate data points from raw values
	 */
	translate: function () {
		var series = this,
			options = series.options,
			yAxis = series.yAxis,
			len,
			i,
			points,
			point,
			shapeArgs,
			stack,
			y,
			yValue,
			previousY,
			previousIntermediate,
			range,
			threshold = options.threshold,
			stacking = options.stacking,
			tooltipY;

		// run column series translate
		seriesTypes.column.prototype.translate.apply(this);

		previousY = previousIntermediate = threshold;
		points = series.points;

		for (i = 0, len = points.length; i < len; i++) {
			// cache current point object
			point = points[i];
			yValue = this.processedYData[i];
			shapeArgs = point.shapeArgs;

			// get current stack
			stack = stacking && yAxis.stacks[(series.negStacks && yValue < threshold ? '-' : '') + series.stackKey];
			range = stack ?
				stack[point.x].points[series.index + ',' + i] :
				[0, yValue];

			// override point value for sums
			// #3710 Update point does not propagate to sum
			if (point.isSum) {
				point.y = yValue;
			} else if (point.isIntermediateSum) {
				point.y = yValue - previousIntermediate; // #3840
			}
			// up points
			y = mathMax(previousY, previousY + point.y) + range[0];
			shapeArgs.y = yAxis.translate(y, 0, 1);


			// sum points
			if (point.isSum) {
				shapeArgs.y = yAxis.translate(range[1], 0, 1);
				shapeArgs.height = Math.min(yAxis.translate(range[0], 0, 1), yAxis.len) - shapeArgs.y; // #4256

			} else if (point.isIntermediateSum) {
				shapeArgs.y = yAxis.translate(range[1], 0, 1);
				shapeArgs.height = Math.min(yAxis.translate(previousIntermediate, 0, 1), yAxis.len) - shapeArgs.y;
				previousIntermediate = range[1];

			// If it's not the sum point, update previous stack end position and get
			// shape height (#3886)
			} else {
				if (previousY !== 0) { // Not the first point
					shapeArgs.height = yValue > 0 ?
						yAxis.translate(previousY, 0, 1) - shapeArgs.y :
						yAxis.translate(previousY, 0, 1) - yAxis.translate(previousY - yValue, 0, 1);
				}
				previousY += yValue;
			}
			// #3952 Negative sum or intermediate sum not rendered correctly
			if (shapeArgs.height < 0) {
				shapeArgs.y += shapeArgs.height;
				shapeArgs.height *= -1;
			}

			point.plotY = shapeArgs.y = mathRound(shapeArgs.y) - (series.borderWidth % 2) / 2;
			shapeArgs.height = mathMax(mathRound(shapeArgs.height), 0.001); // #3151
			point.yBottom = shapeArgs.y + shapeArgs.height;

			// Correct tooltip placement (#3014)
			tooltipY = point.plotY + (point.negative ? shapeArgs.height : 0);
			if (series.chart.inverted) {
				point.tooltipPos[0] = yAxis.len - tooltipY;
			} else {
				point.tooltipPos[1] = tooltipY;
			}

		}
	},

	/**
	 * Call default processData then override yData to reflect waterfall's extremes on yAxis
	 */
	processData: function (force) {
		var series = this,
			options = series.options,
			yData = series.yData,
			points = series.options.data, // #3710 Update point does not propagate to sum
			point,
			dataLength = yData.length,
			threshold = options.threshold || 0,
			subSum,
			sum,
			dataMin,
			dataMax,
			y,
			i;

		sum = subSum = dataMin = dataMax = threshold;

		for (i = 0; i < dataLength; i++) {
			y = yData[i];
			point = points && points[i] ? points[i] : {};

			if (y === 'sum' || point.isSum) {
				yData[i] = sum;
			} else if (y === 'intermediateSum' || point.isIntermediateSum) {
				yData[i] = subSum;
			} else {
				sum += y;
				subSum += y;
			}
			dataMin = Math.min(sum, dataMin);
			dataMax = Math.max(sum, dataMax);
		}

		Series.prototype.processData.call(this, force);

		// Record extremes
		series.dataMin = dataMin;
		series.dataMax = dataMax;
	},

	/**
	 * Return y value or string if point is sum
	 */
	toYData: function (pt) {
		if (pt.isSum) {
			return (pt.x === 0 ? null : 'sum'); //#3245 Error when first element is Sum or Intermediate Sum
		}
		if (pt.isIntermediateSum) {
			return (pt.x === 0 ? null : 'intermediateSum'); //#3245
		}
		return pt.y;
	},

	/**
	 * Postprocess mapping between options and SVG attributes
	 */
	getAttribs: function () {
		seriesTypes.column.prototype.getAttribs.apply(this, arguments);

		var series = this,
			options = series.options,
			stateOptions = options.states,
			upColor = options.upColor || series.color,
			hoverColor = Highcharts.Color(upColor).brighten(0.1).get(),
			seriesDownPointAttr = merge(series.pointAttr),
			upColorProp = series.upColorProp;

		seriesDownPointAttr[''][upColorProp] = upColor;
		seriesDownPointAttr.hover[upColorProp] = stateOptions.hover.upColor || hoverColor;
		seriesDownPointAttr.select[upColorProp] = stateOptions.select.upColor || upColor;

		each(series.points, function (point) {
			if (!point.options.color) {
				// Up color
				if (point.y > 0) {
					point.pointAttr = seriesDownPointAttr;
					point.color = upColor;

				// Down color (#3710, update to negative)
				} else {
					point.pointAttr = series.pointAttr;
				}
			}
		});
	},

	/**
	 * Draw columns' connector lines
	 */
	getGraphPath: function () {

		var data = this.data,
			length = data.length,
			lineWidth = this.options.lineWidth + this.borderWidth,
			normalizer = mathRound(lineWidth) % 2 / 2,
			path = [],
			M = 'M',
			L = 'L',
			prevArgs,
			pointArgs,
			i,
			d;

		for (i = 1; i < length; i++) {
			pointArgs = data[i].shapeArgs;
			prevArgs = data[i - 1].shapeArgs;

			d = [
				M,
				prevArgs.x + prevArgs.width, prevArgs.y + normalizer,
				L,
				pointArgs.x, prevArgs.y + normalizer
			];

			if (data[i - 1].y < 0) {
				d[2] += prevArgs.height;
				d[5] += prevArgs.height;
			}

			path = path.concat(d);
		}

		return path;
	},

	/**
	 * Extremes are recorded in processData
	 */
	getExtremes: noop,

	drawGraph: Series.prototype.drawGraph
});

/* ****************************************************************************
 * End Waterfall series code                                                  *
 *****************************************************************************/
