/*
 * The AreaRangeSeries class
 *
 */

/**
 * Extend the default options with map options
 */
defaultPlotOptions.arearange = merge(defaultPlotOptions.area, {
	lineWidth: 1,
	marker: null,
	threshold: null,
	tooltip: {
		pointFormat: '<span style="color:{series.color}">\u25CF</span> {series.name}: <b>{point.low}</b> - <b>{point.high}</b><br/>'
	},
	trackByArea: true,
	dataLabels: {
		align: null,
		verticalAlign: null,
		xLow: 0,
		xHigh: 0,
		yLow: 0,
		yHigh: 0
	},
	states: {
		hover: {
			halo: false
		}
	}
});

/**
 * Add the series type
 */
seriesTypes.arearange = extendClass(seriesTypes.area, {
	type: 'arearange',
	pointArrayMap: ['low', 'high'],
	dataLabelCollections: ['dataLabel', 'dataLabelUpper'],
	toYData: function (point) {
		return [point.low, point.high];
	},
	pointValKey: 'low',
	deferTranslatePolar: true,

	/**
	 * Translate a point's plotHigh from the internal angle and radius measures to
	 * true plotHigh coordinates. This is an addition of the toXY method found in
	 * Polar.js, because it runs too early for arearanges to be considered (#3419).
	 */
	highToXY: function (point) {
		// Find the polar plotX and plotY
		var chart = this.chart,
			xy = this.xAxis.postTranslate(point.rectPlotX, this.yAxis.len - point.plotHigh);
		point.plotHighX = xy.x - chart.plotLeft;
		point.plotHigh = xy.y - chart.plotTop;
	},

	/**
	 * Extend getSegments to force null points if the higher value is null. #1703.
	 */
	getSegments: function () {
		var series = this;

		each(series.points, function (point) {
			if (!series.options.connectNulls && (point.low === null || point.high === null)) {
				point.y = null;
			} else if (point.low === null && point.high !== null) {
				point.y = point.high;
			}
		});
		Series.prototype.getSegments.call(this);
	},

	/**
	 * Translate data points from raw values x and y to plotX and plotY
	 */
	translate: function () {
		var series = this,
			yAxis = series.yAxis;

		seriesTypes.area.prototype.translate.apply(series);

		// Set plotLow and plotHigh
		each(series.points, function (point) {

			var low = point.low,
				high = point.high,
				plotY = point.plotY;

			if (high === null && low === null) {
				point.y = null;
			} else if (low === null) {
				point.plotLow = point.plotY = null;
				point.plotHigh = yAxis.translate(high, 0, 1, 0, 1);
			} else if (high === null) {
				point.plotLow = plotY;
				point.plotHigh = null;
			} else {
				point.plotLow = plotY;
				point.plotHigh = yAxis.translate(high, 0, 1, 0, 1);
			}
		});

		// Postprocess plotHigh
		if (this.chart.polar) {
			each(this.points, function (point) {
				series.highToXY(point);
			});
		}
	},

	/**
	 * Extend the line series' getSegmentPath method by applying the segment
	 * path to both lower and higher values of the range
	 */
	getSegmentPath: function (segment) {

		var lowSegment,
			highSegment = [],
			i = segment.length,
			baseGetSegmentPath = Series.prototype.getSegmentPath,
			point,
			linePath,
			lowerPath,
			options = this.options,
			step = options.step,
			higherPath;

		// Remove nulls from low segment
		lowSegment = Highcharts.grep(segment, function (point) {
			return point.plotLow !== null;
		});

		// Make a segment with plotX and plotY for the top values
		while (i--) {
			point = segment[i];
			if (point.plotHigh !== null) {
				highSegment.push({
					plotX: point.plotHighX || point.plotX, // plotHighX is for polar charts
					plotY: point.plotHigh
				});
			}
		}

		// Get the paths
		lowerPath = baseGetSegmentPath.call(this, lowSegment);
		if (step) {
			if (step === true) {
				step = 'left';
			}
			options.step = { left: 'right', center: 'center', right: 'left' }[step]; // swap for reading in getSegmentPath
		}
		higherPath = baseGetSegmentPath.call(this, highSegment);
		options.step = step;

		// Create a line on both top and bottom of the range
		linePath = [].concat(lowerPath, higherPath);

		// For the area path, we need to change the 'move' statement into 'lineTo' or 'curveTo'
		if (!this.chart.polar) {
			higherPath[0] = 'L'; // this probably doesn't work for spline
		}
		this.areaPath = this.areaPath.concat(lowerPath, higherPath);

		return linePath;
	},

	/**
	 * Extend the basic drawDataLabels method by running it for both lower and higher
	 * values.
	 */
	drawDataLabels: function () {

		var data = this.data,
			length = data.length,
			i,
			originalDataLabels = [],
			seriesProto = Series.prototype,
			dataLabelOptions = this.options.dataLabels,
			align = dataLabelOptions.align,
			verticalAlign = dataLabelOptions.verticalAlign,
			inside = dataLabelOptions.inside,
			point,
			up,
			inverted = this.chart.inverted;

		if (dataLabelOptions.enabled || this._hasPointLabels) {

			// Step 1: set preliminary values for plotY and dataLabel and draw the upper labels
			i = length;
			while (i--) {
				point = data[i];
				if (point) {
					up = inside ? point.plotHigh < point.plotLow : point.plotHigh > point.plotLow;

					// Set preliminary values
					point.y = point.high;
					point._plotY = point.plotY;
					point.plotY = point.plotHigh;

					// Store original data labels and set preliminary label objects to be picked up
					// in the uber method
					originalDataLabels[i] = point.dataLabel;
					point.dataLabel = point.dataLabelUpper;

					// Set the default offset
					point.below = up;
					if (inverted) {
						if (!align) {
							dataLabelOptions.align = up ? 'right' : 'left';
						}
					} else {
						if (!verticalAlign) {
							dataLabelOptions.verticalAlign = up ? 'top' : 'bottom';
						}
					}

					dataLabelOptions.x = dataLabelOptions.xHigh;
					dataLabelOptions.y = dataLabelOptions.yHigh;
				}
			}

			if (seriesProto.drawDataLabels) {
				seriesProto.drawDataLabels.apply(this, arguments); // #1209
			}

			// Step 2: reorganize and handle data labels for the lower values
			i = length;
			while (i--) {
				point = data[i];
				if (point) {
					up = inside ? point.plotHigh < point.plotLow : point.plotHigh > point.plotLow;

					// Move the generated labels from step 1, and reassign the original data labels
					point.dataLabelUpper = point.dataLabel;
					point.dataLabel = originalDataLabels[i];

					// Reset values
					point.y = point.low;
					point.plotY = point._plotY;

					// Set the default offset
					point.below = !up;
					if (inverted) {
						if (!align) {
							dataLabelOptions.align = up ? 'left' : 'right';
						}
					} else {
						if (!verticalAlign) {
							dataLabelOptions.verticalAlign = up ? 'bottom' : 'top';
						}
						
					}

					dataLabelOptions.x = dataLabelOptions.xLow;
					dataLabelOptions.y = dataLabelOptions.yLow;
				}
			}
			if (seriesProto.drawDataLabels) {
				seriesProto.drawDataLabels.apply(this, arguments);
			}
		}

		dataLabelOptions.align = align;
		dataLabelOptions.verticalAlign = verticalAlign;
	},

	alignDataLabel: function () {
		seriesTypes.column.prototype.alignDataLabel.apply(this, arguments);
	},

	setStackedPoints: noop,

	getSymbol: noop,

	drawPoints: noop
});
