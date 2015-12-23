/**
 * Create a new axis object
 * @param {Object} chart
 * @param {Object} options
 */
var Axis = Highcharts.Axis = function () {
	this.init.apply(this, arguments);
};

Axis.prototype = {

	/**
	 * Default options for the X axis - the Y axis has extended defaults
	 */
	defaultOptions: {
		// allowDecimals: null,
		// alternateGridColor: null,
		// categories: [],
		dateTimeLabelFormats: {
			millisecond: '%H:%M:%S.%L',
			second: '%H:%M:%S',
			minute: '%H:%M',
			hour: '%H:%M',
			day: '%e. %b',
			week: '%e. %b',
			month: '%b \'%y',
			year: '%Y'
		},
		endOnTick: false,
		gridLineColor: '#D8D8D8',
		// gridLineDashStyle: 'solid',
		// gridLineWidth: 0,
		// reversed: false,

		labels: {
			enabled: true,
			// rotation: 0,
			// align: 'center',
			// step: null,
			style: {
				color: '#606060',
				cursor: 'default',
				fontSize: '11px'
			},
			x: 0,
			y: 15
			/*formatter: function () {
				return this.value;
			},*/
		},
		lineColor: '#C0D0E0',
		lineWidth: 1,
		//linkedTo: null,
		//max: undefined,
		//min: undefined,
		minPadding: 0.01,
		maxPadding: 0.01,
		//minRange: null,
		minorGridLineColor: '#E0E0E0',
		// minorGridLineDashStyle: null,
		minorGridLineWidth: 1,
		minorTickColor: '#A0A0A0',
		//minorTickInterval: null,
		minorTickLength: 2,
		minorTickPosition: 'outside', // inside or outside
		//minorTickWidth: 0,
		//opposite: false,
		//offset: 0,
		//plotBands: [{
		//	events: {},
		//	zIndex: 1,
		//	labels: { align, x, verticalAlign, y, style, rotation, textAlign }
		//}],
		//plotLines: [{
		//	events: {}
		//  dashStyle: {}
		//	zIndex:
		//	labels: { align, x, verticalAlign, y, style, rotation, textAlign }
		//}],
		//reversed: false,
		// showFirstLabel: true,
		// showLastLabel: true,
		startOfWeek: 1,
		startOnTick: false,
		tickColor: '#C0D0E0',
		//tickInterval: null,
		tickLength: 10,
		tickmarkPlacement: 'between', // on or between
		tickPixelInterval: 100,
		tickPosition: 'outside',
		//tickWidth: 1,
		title: {
			//text: null,
			align: 'middle', // low, middle or high
			//margin: 0 for horizontal, 10 for vertical axes,
			//rotation: 0,
			//side: 'outside',
			style: {
				color: '#707070'
			}
			//x: 0,
			//y: 0
		},
		type: 'linear' // linear, logarithmic or datetime
		//visible: true
	},

	/**
	 * This options set extends the defaultOptions for Y axes
	 */
	defaultYAxisOptions: {
		endOnTick: true,
		gridLineWidth: 1,
		tickPixelInterval: 72,
		showLastLabel: true,
		labels: {
			x: -8,
			y: 3
		},
		lineWidth: 0,
		maxPadding: 0.05,
		minPadding: 0.05,
		startOnTick: true,
		//tickWidth: 0,
		title: {
			rotation: 270,
			text: 'Values'
		},
		stackLabels: {
			enabled: false,
			//align: dynamic,
			//y: dynamic,
			//x: dynamic,
			//verticalAlign: dynamic,
			//textAlign: dynamic,
			//rotation: 0,
			formatter: function () {
				return Highcharts.numberFormat(this.total, -1);
			},
			style: merge(defaultPlotOptions.line.dataLabels.style, { color: '#000000' })
		}
	},

	/**
	 * These options extend the defaultOptions for left axes
	 */
	defaultLeftAxisOptions: {
		labels: {
			x: -15,
			y: null
		},
		title: {
			rotation: 270
		}
	},

	/**
	 * These options extend the defaultOptions for right axes
	 */
	defaultRightAxisOptions: {
		labels: {
			x: 15,
			y: null
		},
		title: {
			rotation: 90
		}
	},

	/**
	 * These options extend the defaultOptions for bottom axes
	 */
	defaultBottomAxisOptions: {
		labels: {
			autoRotation: [-45],
			x: 0,
			y: null // based on font size
			// overflow: undefined,
			// staggerLines: null
		},
		title: {
			rotation: 0
		}
	},
	/**
	 * These options extend the defaultOptions for top axes
	 */
	defaultTopAxisOptions: {
		labels: {
			autoRotation: [-45],
			x: 0,
			y: -15
			// overflow: undefined
			// staggerLines: null
		},
		title: {
			rotation: 0
		}
	},

	/**
	 * Initialize the axis
	 */
	init: function (chart, userOptions) {


		var isXAxis = userOptions.isX,
			axis = this;

		axis.chart = chart;

		// Flag, is the axis horizontal
		axis.horiz = chart.inverted ? !isXAxis : isXAxis;

		// Flag, isXAxis
		axis.isXAxis = isXAxis;
		axis.coll = isXAxis ? 'xAxis' : 'yAxis';

		axis.opposite = userOptions.opposite; // needed in setOptions
		axis.side = userOptions.side || (axis.horiz ?
				(axis.opposite ? 0 : 2) : // top : bottom
				(axis.opposite ? 1 : 3));  // right : left

		axis.setOptions(userOptions);


		var options = this.options,
			type = options.type,
			isDatetimeAxis = type === 'datetime';

		axis.labelFormatter = options.labels.formatter || axis.defaultLabelFormatter; // can be overwritten by dynamic format


		// Flag, stagger lines or not
		axis.userOptions = userOptions;

		//axis.axisTitleMargin = UNDEFINED,// = options.title.margin,
		axis.minPixelPadding = 0;

		axis.reversed = options.reversed;
		axis.visible = options.visible !== false;
		axis.zoomEnabled = options.zoomEnabled !== false;

		// Initial categories
		axis.categories = options.categories || type === 'category';
		axis.names = axis.names || []; // Preserve on update (#3830)

		// Elements
		//axis.axisGroup = UNDEFINED;
		//axis.gridGroup = UNDEFINED;
		//axis.axisTitle = UNDEFINED;
		//axis.axisLine = UNDEFINED;

		// Shorthand types
		axis.isLog = type === 'logarithmic';
		axis.isDatetimeAxis = isDatetimeAxis;

		// Flag, if axis is linked to another axis
		axis.isLinked = defined(options.linkedTo);
		// Linked axis.
		//axis.linkedParent = UNDEFINED;

		// Tick positions
		//axis.tickPositions = UNDEFINED; // array containing predefined positions
		// Tick intervals
		//axis.tickInterval = UNDEFINED;
		//axis.minorTickInterval = UNDEFINED;


		// Major ticks
		axis.ticks = {};
		axis.labelEdge = [];
		// Minor ticks
		axis.minorTicks = {};

		// List of plotLines/Bands
		axis.plotLinesAndBands = [];

		// Alternate bands
		axis.alternateBands = {};

		// Axis metrics
		//axis.left = UNDEFINED;
		//axis.top = UNDEFINED;
		//axis.width = UNDEFINED;
		//axis.height = UNDEFINED;
		//axis.bottom = UNDEFINED;
		//axis.right = UNDEFINED;
		//axis.transA = UNDEFINED;
		//axis.transB = UNDEFINED;
		//axis.oldTransA = UNDEFINED;
		axis.len = 0;
		//axis.oldMin = UNDEFINED;
		//axis.oldMax = UNDEFINED;
		//axis.oldUserMin = UNDEFINED;
		//axis.oldUserMax = UNDEFINED;
		//axis.oldAxisLength = UNDEFINED;
		axis.minRange = axis.userMinRange = options.minRange || options.maxZoom;
		axis.range = options.range;
		axis.offset = options.offset || 0;


		// Dictionary for stacks
		axis.stacks = {};
		axis.oldStacks = {};
		axis.stacksTouched = 0;

		// Min and max in the data
		//axis.dataMin = UNDEFINED,
		//axis.dataMax = UNDEFINED,

		// The axis range
		axis.max = null;
		axis.min = null;

		// User set min and max
		//axis.userMin = UNDEFINED,
		//axis.userMax = UNDEFINED,

		// Crosshair options
		axis.crosshair = pick(options.crosshair, splat(chart.options.tooltip.crosshairs)[isXAxis ? 0 : 1], false);
		// Run Axis

		var eventType,
			events = axis.options.events;

		// Register
		if (inArray(axis, chart.axes) === -1) { // don't add it again on Axis.update()
			if (isXAxis && !this.isColorAxis) { // #2713
				chart.axes.splice(chart.xAxis.length, 0, axis);
			} else {
				chart.axes.push(axis);
			}

			chart[axis.coll].push(axis);
		}

		axis.series = axis.series || []; // populated by Series

		// inverted charts have reversed xAxes as default
		if (chart.inverted && isXAxis && axis.reversed === UNDEFINED) {
			axis.reversed = true;
		}

		axis.removePlotBand = axis.removePlotBandOrLine;
		axis.removePlotLine = axis.removePlotBandOrLine;


		// register event listeners
		for (eventType in events) {
			addEvent(axis, eventType, events[eventType]);
		}

		// extend logarithmic axis
		if (axis.isLog) {
			axis.val2lin = log2lin;
			axis.lin2val = lin2log;
		}
	},

	/**
	 * Merge and set options
	 */
	setOptions: function (userOptions) {
		this.options = merge(
			this.defaultOptions,
			this.isXAxis ? {} : this.defaultYAxisOptions,
			[this.defaultTopAxisOptions, this.defaultRightAxisOptions,
				this.defaultBottomAxisOptions, this.defaultLeftAxisOptions][this.side],
			merge(
				defaultOptions[this.coll], // if set in setOptions (#1053)
				userOptions
			)
		);
	},

	/**
	 * The default label formatter. The context is a special config object for the label.
	 */
	defaultLabelFormatter: function () {
		var axis = this.axis,
			value = this.value,
			categories = axis.categories,
			dateTimeLabelFormat = this.dateTimeLabelFormat,
			numericSymbols = defaultOptions.lang.numericSymbols,
			i = numericSymbols && numericSymbols.length,
			multi,
			ret,
			formatOption = axis.options.labels.format,

			// make sure the same symbol is added for all labels on a linear axis
			numericSymbolDetector = axis.isLog ? value : axis.tickInterval;

		if (formatOption) {
			ret = format(formatOption, this);

		} else if (categories) {
			ret = value;

		} else if (dateTimeLabelFormat) { // datetime axis
			ret = dateFormat(dateTimeLabelFormat, value);

		} else if (i && numericSymbolDetector >= 1000) {
			// Decide whether we should add a numeric symbol like k (thousands) or M (millions).
			// If we are to enable this in tooltip or other places as well, we can move this
			// logic to the numberFormatter and enable it by a parameter.
			while (i-- && ret === UNDEFINED) {
				multi = Math.pow(1000, i + 1);
				if (numericSymbolDetector >= multi && (value * 10) % multi === 0 && numericSymbols[i] !== null) {
					ret = Highcharts.numberFormat(value / multi, -1) + numericSymbols[i];
				}
			}
		}

		if (ret === UNDEFINED) {
			if (mathAbs(value) >= 10000) { // add thousands separators
				ret = Highcharts.numberFormat(value, -1);

			} else { // small numbers
				ret = Highcharts.numberFormat(value, -1, UNDEFINED, ''); // #2466
			}
		}

		return ret;
	},

	/**
	 * Get the minimum and maximum for the series of each axis
	 */
	getSeriesExtremes: function () {
		var axis = this,
			chart = axis.chart;

		axis.hasVisibleSeries = false;

		// Reset properties in case we're redrawing (#3353)
		axis.dataMin = axis.dataMax = axis.threshold = null;
		axis.softThreshold = !axis.isXAxis;

		if (axis.buildStacks) {
			axis.buildStacks();
		}

		// loop through this axis' series
		each(axis.series, function (series) {

			if (series.visible || !chart.options.chart.ignoreHiddenSeries) {

				var seriesOptions = series.options,
					xData,
					threshold = seriesOptions.threshold,
					seriesDataMin,
					seriesDataMax;

				axis.hasVisibleSeries = true;

				// Validate threshold in logarithmic axes
				if (axis.isLog && threshold <= 0) {
					threshold = null;
				}

				// Get dataMin and dataMax for X axes
				if (axis.isXAxis) {
					xData = series.xData;
					if (xData.length) {
						axis.dataMin = mathMin(pick(axis.dataMin, xData[0]), arrayMin(xData));
						axis.dataMax = mathMax(pick(axis.dataMax, xData[0]), arrayMax(xData));
					}

				// Get dataMin and dataMax for Y axes, as well as handle stacking and processed data
				} else {

					// Get this particular series extremes
					series.getExtremes();
					seriesDataMax = series.dataMax;
					seriesDataMin = series.dataMin;

					// Get the dataMin and dataMax so far. If percentage is used, the min and max are
					// always 0 and 100. If seriesDataMin and seriesDataMax is null, then series
					// doesn't have active y data, we continue with nulls
					if (defined(seriesDataMin) && defined(seriesDataMax)) {
						axis.dataMin = mathMin(pick(axis.dataMin, seriesDataMin), seriesDataMin);
						axis.dataMax = mathMax(pick(axis.dataMax, seriesDataMax), seriesDataMax);
					}

					// Adjust to threshold
					if (defined(threshold)) {
						axis.threshold = threshold;
					}
					// If any series has a hard threshold, it takes precedence
					if (!seriesOptions.softThreshold || axis.isLog) {
						axis.softThreshold = false;
					}
				}
			}
		});
	},

	/**
	 * Translate from axis value to pixel position on the chart, or back
	 *
	 */
	translate: function (val, backwards, cvsCoord, old, handleLog, pointPlacement) {
		var axis = this.linkedParent || this, // #1417
			sign = 1,
			cvsOffset = 0,
			localA = old ? axis.oldTransA : axis.transA,
			localMin = old ? axis.oldMin : axis.min,
			returnValue,
			minPixelPadding = axis.minPixelPadding,
			doPostTranslate = (axis.doPostTranslate || (axis.isLog && handleLog)) && axis.lin2val;

		if (!localA) {
			localA = axis.transA;
		}

		// In vertical axes, the canvas coordinates start from 0 at the top like in
		// SVG.
		if (cvsCoord) {
			sign *= -1; // canvas coordinates inverts the value
			cvsOffset = axis.len;
		}

		// Handle reversed axis
		if (axis.reversed) {
			sign *= -1;
			cvsOffset -= sign * (axis.sector || axis.len);
		}

		// From pixels to value
		if (backwards) { // reverse translation

			val = val * sign + cvsOffset;
			val -= minPixelPadding;
			returnValue = val / localA + localMin; // from chart pixel to value
			if (doPostTranslate) { // log and ordinal axes
				returnValue = axis.lin2val(returnValue);
			}

		// From value to pixels
		} else {
			if (doPostTranslate) { // log and ordinal axes
				val = axis.val2lin(val);
			}
			if (pointPlacement === 'between') {
				pointPlacement = 0.5;
			}
			returnValue = sign * (val - localMin) * localA + cvsOffset + (sign * minPixelPadding) +
				(isNumber(pointPlacement) ? localA * pointPlacement * axis.pointRange : 0);
		}

		return returnValue;
	},

	/**
	 * Utility method to translate an axis value to pixel position.
	 * @param {Number} value A value in terms of axis units
	 * @param {Boolean} paneCoordinates Whether to return the pixel coordinate relative to the chart
	 *        or just the axis/pane itself.
	 */
	toPixels: function (value, paneCoordinates) {
		return this.translate(value, false, !this.horiz, null, true) + (paneCoordinates ? 0 : this.pos);
	},

	/*
	 * Utility method to translate a pixel position in to an axis value
	 * @param {Number} pixel The pixel value coordinate
	 * @param {Boolean} paneCoordiantes Whether the input pixel is relative to the chart or just the
	 *        axis/pane itself.
	 */
	toValue: function (pixel, paneCoordinates) {
		return this.translate(pixel - (paneCoordinates ? 0 : this.pos), true, !this.horiz, null, true);
	},

	/**
	 * Create the path for a plot line that goes from the given value on
	 * this axis, across the plot to the opposite side
	 * @param {Number} value
	 * @param {Number} lineWidth Used for calculation crisp line
	 * @param {Number] old Use old coordinates (for resizing and rescaling)
	 */
	getPlotLinePath: function (value, lineWidth, old, force, translatedValue) {
		var axis = this,
			chart = axis.chart,
			axisLeft = axis.left,
			axisTop = axis.top,
			x1,
			y1,
			x2,
			y2,
			cHeight = (old && chart.oldChartHeight) || chart.chartHeight,
			cWidth = (old && chart.oldChartWidth) || chart.chartWidth,
			skip,
			transB = axis.transB,
			/**
			 * Check if x is between a and b. If not, either move to a/b or skip,
			 * depending on the force parameter.
			 */
			between = function (x, a, b) {
				if (x < a || x > b) {
					if (force) {
						x = mathMin(mathMax(a, x), b);
					} else {
						skip = true;
					}
				}
				return x;
			};

		translatedValue = pick(translatedValue, axis.translate(value, null, null, old));
		x1 = x2 = mathRound(translatedValue + transB);
		y1 = y2 = mathRound(cHeight - translatedValue - transB);

		if (isNaN(translatedValue)) { // no min or max
			skip = true;

		} else if (axis.horiz) {
			y1 = axisTop;
			y2 = cHeight - axis.bottom;
			x1 = x2 = between(x1, axisLeft, axisLeft + axis.width);
		} else {
			x1 = axisLeft;
			x2 = cWidth - axis.right;
			y1 = y2 = between(y1, axisTop, axisTop + axis.height);
		}
		return skip && !force ?
			null :
			chart.renderer.crispLine([M, x1, y1, L, x2, y2], lineWidth || 1);
	},

	/**
	 * Set the tick positions of a linear axis to round values like whole tens or every five.
	 */
	getLinearTickPositions: function (tickInterval, min, max) {
		var pos,
			lastPos,
			roundedMin = correctFloat(mathFloor(min / tickInterval) * tickInterval),
			roundedMax = correctFloat(mathCeil(max / tickInterval) * tickInterval),
			tickPositions = [];

		// For single points, add a tick regardless of the relative position (#2662)
		if (min === max && isNumber(min)) {
			return [min];
		}

		// Populate the intermediate values
		pos = roundedMin;
		while (pos <= roundedMax) {

			// Place the tick on the rounded value
			tickPositions.push(pos);

			// Always add the raw tickInterval, not the corrected one.
			pos = correctFloat(pos + tickInterval);

			// If the interval is not big enough in the current min - max range to actually increase
			// the loop variable, we need to break out to prevent endless loop. Issue #619
			if (pos === lastPos) {
				break;
			}

			// Record the last value
			lastPos = pos;
		}
		return tickPositions;
	},

	/**
	 * Return the minor tick positions. For logarithmic axes, reuse the same logic
	 * as for major ticks.
	 */
	getMinorTickPositions: function () {
		var axis = this,
			options = axis.options,
			tickPositions = axis.tickPositions,
			minorTickInterval = axis.minorTickInterval,
			minorTickPositions = [],
			pos,
			i,
			pointRangePadding = axis.pointRangePadding || 0,
			min = axis.min - pointRangePadding, // #1498
			max = axis.max + pointRangePadding, // #1498
			range = max - min,
			len;

		// If minor ticks get too dense, they are hard to read, and may cause long running script. So we don't draw them.
		if (range && range / minorTickInterval < axis.len / 3) { // #3875

			if (axis.isLog) {
				len = tickPositions.length;
				for (i = 1; i < len; i++) {
					minorTickPositions = minorTickPositions.concat(
						axis.getLogTickPositions(minorTickInterval, tickPositions[i - 1], tickPositions[i], true)
					);
				}
			} else if (axis.isDatetimeAxis && options.minorTickInterval === 'auto') { // #1314
				minorTickPositions = minorTickPositions.concat(
					axis.getTimeTicks(
						axis.normalizeTimeTickInterval(minorTickInterval),
						min,
						max,
						options.startOfWeek
					)
				);
			} else {
				for (pos = min + (tickPositions[0] - min) % minorTickInterval; pos <= max; pos += minorTickInterval) {
					minorTickPositions.push(pos);
				}
			}
		}

		if (minorTickPositions.length !== 0) { // don't change the extremes, when there is no minor ticks
			axis.trimTicks(minorTickPositions, options.startOnTick, options.endOnTick); // #3652 #3743 #1498
		}
		return minorTickPositions;
	},

	/**
	 * Adjust the min and max for the minimum range. Keep in mind that the series data is
	 * not yet processed, so we don't have information on data cropping and grouping, or
	 * updated axis.pointRange or series.pointRange. The data can't be processed until
	 * we have finally established min and max.
	 */
	adjustForMinRange: function () {
		var axis = this,
			options = axis.options,
			min = axis.min,
			max = axis.max,
			zoomOffset,
			spaceAvailable = axis.dataMax - axis.dataMin >= axis.minRange,
			closestDataRange,
			i,
			distance,
			xData,
			loopLength,
			minArgs,
			maxArgs,
			minRange;

		// Set the automatic minimum range based on the closest point distance
		if (axis.isXAxis && axis.minRange === UNDEFINED && !axis.isLog) {

			if (defined(options.min) || defined(options.max)) {
				axis.minRange = null; // don't do this again

			} else {

				// Find the closest distance between raw data points, as opposed to
				// closestPointRange that applies to processed points (cropped and grouped)
				each(axis.series, function (series) {
					xData = series.xData;
					loopLength = series.xIncrement ? 1 : xData.length - 1;
					for (i = loopLength; i > 0; i--) {
						distance = xData[i] - xData[i - 1];
						if (closestDataRange === UNDEFINED || distance < closestDataRange) {
							closestDataRange = distance;
						}
					}
				});
				axis.minRange = mathMin(closestDataRange * 5, axis.dataMax - axis.dataMin);
			}
		}

		// if minRange is exceeded, adjust
		if (max - min < axis.minRange) {
			minRange = axis.minRange;
			zoomOffset = (minRange - max + min) / 2;

			// if min and max options have been set, don't go beyond it
			minArgs = [min - zoomOffset, pick(options.min, min - zoomOffset)];
			if (spaceAvailable) { // if space is available, stay within the data range
				minArgs[2] = axis.dataMin;
			}
			min = arrayMax(minArgs);

			maxArgs = [min + minRange, pick(options.max, min + minRange)];
			if (spaceAvailable) { // if space is availabe, stay within the data range
				maxArgs[2] = axis.dataMax;
			}

			max = arrayMin(maxArgs);

			// now if the max is adjusted, adjust the min back
			if (max - min < minRange) {
				minArgs[0] = max - minRange;
				minArgs[1] = pick(options.min, max - minRange);
				min = arrayMax(minArgs);
			}
		}

		// Record modified extremes
		axis.min = min;
		axis.max = max;
	},

	/**
	 * Update translation information
	 */
	setAxisTranslation: function (saveOld) {
		var axis = this,
			range = axis.max - axis.min,
			pointRange = axis.axisPointRange || 0,
			closestPointRange,
			minPointOffset = 0,
			pointRangePadding = 0,
			linkedParent = axis.linkedParent,
			ordinalCorrection,
			hasCategories = !!axis.categories,
			transA = axis.transA,
			isXAxis = axis.isXAxis;

		// Adjust translation for padding. Y axis with categories need to go through the same (#1784).
		if (isXAxis || hasCategories || pointRange) {
			if (linkedParent) {
				minPointOffset = linkedParent.minPointOffset;
				pointRangePadding = linkedParent.pointRangePadding;

			} else {
				// Find the closestPointRange across all series
				each(axis.series, function (series) {
					var seriesClosest = series.closestPointRange;
					if (!series.noSharedTooltip && defined(seriesClosest)) {
						closestPointRange = defined(closestPointRange) ?
							mathMin(closestPointRange, seriesClosest) :
							seriesClosest;
					}
				});

				each(axis.series, function (series) {
					var seriesPointRange = hasCategories ? 
						1 : 
						(isXAxis ? 
							pick(series.options.pointRange, closestPointRange, 0) : 
							(axis.axisPointRange || 0)), // #2806
						pointPlacement = series.options.pointPlacement;

					pointRange = mathMax(pointRange, seriesPointRange);

					if (!axis.single) {
						// minPointOffset is the value padding to the left of the axis in order to make
						// room for points with a pointRange, typically columns. When the pointPlacement option
						// is 'between' or 'on', this padding does not apply.
						minPointOffset = mathMax(
							minPointOffset,
							isString(pointPlacement) ? 0 : seriesPointRange / 2
						);

						// Determine the total padding needed to the length of the axis to make room for the
						// pointRange. If the series' pointPlacement is 'on', no padding is added.
						pointRangePadding = mathMax(
							pointRangePadding,
							pointPlacement === 'on' ? 0 : seriesPointRange
						);
					}
				});
			}

			// Record minPointOffset and pointRangePadding
			ordinalCorrection = axis.ordinalSlope && closestPointRange ? axis.ordinalSlope / closestPointRange : 1; // #988, #1853
			axis.minPointOffset = minPointOffset = minPointOffset * ordinalCorrection;
			axis.pointRangePadding = pointRangePadding = pointRangePadding * ordinalCorrection;

			// pointRange means the width reserved for each point, like in a column chart
			axis.pointRange = mathMin(pointRange, range);

			// closestPointRange means the closest distance between points. In columns
			// it is mostly equal to pointRange, but in lines pointRange is 0 while closestPointRange
			// is some other value
			if (isXAxis) {
				axis.closestPointRange = closestPointRange;
			}
		}

		// Secondary values
		if (saveOld) {
			axis.oldTransA = transA;
		}
		axis.translationSlope = axis.transA = transA = axis.len / ((range + pointRangePadding) || 1);
		axis.transB = axis.horiz ? axis.left : axis.bottom; // translation addend
		axis.minPixelPadding = transA * minPointOffset;
	},

	minFromRange: function () {
		return this.max - this.range;
	},

	/**
	 * Set the tick positions to round values and optionally extend the extremes
	 * to the nearest tick
	 */
	setTickInterval: function (secondPass) {
		var axis = this,
			chart = axis.chart,
			options = axis.options,
			isLog = axis.isLog,
			isDatetimeAxis = axis.isDatetimeAxis,
			isXAxis = axis.isXAxis,
			isLinked = axis.isLinked,
			maxPadding = options.maxPadding,
			minPadding = options.minPadding,
			length,
			linkedParentExtremes,
			tickIntervalOption = options.tickInterval,
			minTickInterval,
			tickPixelIntervalOption = options.tickPixelInterval,
			categories = axis.categories,
			threshold = axis.threshold,
			softThreshold = axis.softThreshold,
			thresholdMin,
			thresholdMax,
			hardMin,
			hardMax;

		if (!isDatetimeAxis && !categories && !isLinked) {
			this.getTickAmount();
		}

		// Min or max set either by zooming/setExtremes or initial options
		hardMin = pick(axis.userMin, options.min);
		hardMax = pick(axis.userMax, options.max);

		// Linked axis gets the extremes from the parent axis
		if (isLinked) {
			axis.linkedParent = chart[axis.coll][options.linkedTo];
			linkedParentExtremes = axis.linkedParent.getExtremes();
			axis.min = pick(linkedParentExtremes.min, linkedParentExtremes.dataMin);
			axis.max = pick(linkedParentExtremes.max, linkedParentExtremes.dataMax);
			if (options.type !== axis.linkedParent.options.type) {
				error(11, 1); // Can't link axes of different type
			}

		// Initial min and max from the extreme data values
		} else {

			// Adjust to hard threshold
			if (!softThreshold && defined(threshold)) {
				if (axis.dataMin >= threshold) {
					thresholdMin = threshold;
					minPadding = 0;
				} else if (axis.dataMax <= threshold) {
					thresholdMax = threshold;
					maxPadding = 0;
				}
			}

			axis.min = pick(hardMin, thresholdMin, axis.dataMin);
			axis.max = pick(hardMax, thresholdMax, axis.dataMax);

		}

		if (isLog) {
			if (!secondPass && mathMin(axis.min, pick(axis.dataMin, axis.min)) <= 0) { // #978
				error(10, 1); // Can't plot negative values on log axis
			}
			// The correctFloat cures #934, float errors on full tens. But it
			// was too aggressive for #4360 because of conversion back to lin,
			// therefore use precision 15.
			axis.min = correctFloat(log2lin(axis.min), 15);
			axis.max = correctFloat(log2lin(axis.max), 15);
		}

		// handle zoomed range
		if (axis.range && defined(axis.max)) {
			axis.userMin = axis.min = hardMin = mathMax(axis.min, axis.minFromRange()); // #618
			axis.userMax = hardMax = axis.max;

			axis.range = null;  // don't use it when running setExtremes
		}

		// Hook for adjusting this.min and this.max. Used by bubble series.
		if (axis.beforePadding) {
			axis.beforePadding();
		}

		// adjust min and max for the minimum range
		axis.adjustForMinRange();

		// Pad the values to get clear of the chart's edges. To avoid tickInterval taking the padding
		// into account, we do this after computing tick interval (#1337).
		if (!categories && !axis.axisPointRange && !axis.usePercentage && !isLinked && defined(axis.min) && defined(axis.max)) {
			length = axis.max - axis.min;
			if (length) {
				if (!defined(hardMin) && minPadding) {
					axis.min -= length * minPadding;
				}
				if (!defined(hardMax)  && maxPadding) {
					axis.max += length * maxPadding;
				}
			}
		}

		// Stay within floor and ceiling
		if (isNumber(options.floor)) {
			axis.min = mathMax(axis.min, options.floor);
		}
		if (isNumber(options.ceiling)) {
			axis.max = mathMin(axis.max, options.ceiling);
		}

		// When the threshold is soft, adjust the extreme value only if
		// the data extreme and the padded extreme land on either side of the threshold. For example,
		// a series of [0, 1, 2, 3] would make the yAxis add a tick for -1 because of the
		// default minPadding and startOnTick options. This is prevented by the softThreshold
		// option.
		if (softThreshold && defined(axis.dataMin)) {
			threshold = threshold || 0;
			if (!defined(hardMin) && axis.min < threshold && axis.dataMin >= threshold) {
				axis.min = threshold;
			} else if (!defined(hardMax) && axis.max > threshold && axis.dataMax <= threshold) {
				axis.max = threshold;
			}
		}


		// get tickInterval
		if (axis.min === axis.max || axis.min === undefined || axis.max === undefined) {
			axis.tickInterval = 1;
		} else if (isLinked && !tickIntervalOption &&
				tickPixelIntervalOption === axis.linkedParent.options.tickPixelInterval) {
			axis.tickInterval = tickIntervalOption = axis.linkedParent.tickInterval;
		} else {
			axis.tickInterval = pick(
				tickIntervalOption,
				this.tickAmount ? ((axis.max - axis.min) / mathMax(this.tickAmount - 1, 1)) : undefined,
				categories ? // for categoried axis, 1 is default, for linear axis use tickPix
					1 :
					// don't let it be more than the data range
					(axis.max - axis.min) * tickPixelIntervalOption / mathMax(axis.len, tickPixelIntervalOption)
			);
		}

		// Now we're finished detecting min and max, crop and group series data. This
		// is in turn needed in order to find tick positions in ordinal axes.
		if (isXAxis && !secondPass) {
			each(axis.series, function (series) {
				series.processData(axis.min !== axis.oldMin || axis.max !== axis.oldMax);
			});
		}

		// set the translation factor used in translate function
		axis.setAxisTranslation(true);

		// hook for ordinal axes and radial axes
		if (axis.beforeSetTickPositions) {
			axis.beforeSetTickPositions();
		}

		// hook for extensions, used in Highstock ordinal axes
		if (axis.postProcessTickInterval) {
			axis.tickInterval = axis.postProcessTickInterval(axis.tickInterval);
		}

		// In column-like charts, don't cramp in more ticks than there are points (#1943, #4184)
		if (axis.pointRange && !tickIntervalOption) {
			axis.tickInterval = mathMax(axis.pointRange, axis.tickInterval);
		}

		// Before normalizing the tick interval, handle minimum tick interval. This applies only if tickInterval is not defined.
		minTickInterval = pick(options.minTickInterval, axis.isDatetimeAxis && axis.closestPointRange);
		if (!tickIntervalOption && axis.tickInterval < minTickInterval) {
			axis.tickInterval = minTickInterval;
		}

		// for linear axes, get magnitude and normalize the interval
		if (!isDatetimeAxis && !isLog && !tickIntervalOption) {
			axis.tickInterval = normalizeTickInterval(
				axis.tickInterval,
				null,
				getMagnitude(axis.tickInterval),
				// If the tick interval is between 0.5 and 5 and the axis max is in the order of
				// thousands, chances are we are dealing with years. Don't allow decimals. #3363.
				pick(options.allowDecimals, !(axis.tickInterval > 0.5 && axis.tickInterval < 5 && axis.max > 1000 && axis.max < 9999)),
				!!this.tickAmount
			);
		}

		// Prevent ticks from getting so close that we can't draw the labels
		if (!this.tickAmount && this.len) { // Color axis with disabled legend has no length
			axis.tickInterval = axis.unsquish();
		}

		this.setTickPositions();
	},

	/**
	 * Now we have computed the normalized tickInterval, get the tick positions
	 */
	setTickPositions: function () {

		var options = this.options,
			tickPositions,
			tickPositionsOption = options.tickPositions,
			tickPositioner = options.tickPositioner,
			startOnTick = options.startOnTick,
			endOnTick = options.endOnTick,
			single;

		// Set the tickmarkOffset
		this.tickmarkOffset = (this.categories && options.tickmarkPlacement === 'between' &&
			this.tickInterval === 1) ? 0.5 : 0; // #3202


		// get minorTickInterval
		this.minorTickInterval = options.minorTickInterval === 'auto' && this.tickInterval ?
			this.tickInterval / 5 : options.minorTickInterval;

		// Find the tick positions
		this.tickPositions = tickPositions = tickPositionsOption && tickPositionsOption.slice(); // Work on a copy (#1565)
		if (!tickPositions) {

			if (this.isDatetimeAxis) {
				tickPositions = this.getTimeTicks(
					this.normalizeTimeTickInterval(this.tickInterval, options.units),
					this.min,
					this.max,
					options.startOfWeek,
					this.ordinalPositions,
					this.closestPointRange,
					true
				);
			} else if (this.isLog) {
				tickPositions = this.getLogTickPositions(this.tickInterval, this.min, this.max);
			} else {
				tickPositions = this.getLinearTickPositions(this.tickInterval, this.min, this.max);
			}

			// Too dense ticks, keep only the first and last (#4477)
			if (tickPositions.length > this.len) {
				tickPositions = [tickPositions[0], tickPositions.pop()];
			}

			this.tickPositions = tickPositions;

			// Run the tick positioner callback, that allows modifying auto tick positions.
			if (tickPositioner) {
				tickPositioner = tickPositioner.apply(this, [this.min, this.max]);
				if (tickPositioner) {
					this.tickPositions = tickPositions = tickPositioner;
				}
			}

		}

		if (!this.isLinked) {

			// reset min/max or remove extremes based on start/end on tick
			this.trimTicks(tickPositions, startOnTick, endOnTick);

			// When there is only one point, or all points have the same value on this axis, then min
			// and max are equal and tickPositions.length is 0 or 1. In this case, add some padding
			// in order to center the point, but leave it with one tick. #1337.
			if (this.min === this.max && defined(this.min) && !this.tickAmount) {
				// Substract half a unit (#2619, #2846, #2515, #3390)
				single = true;
				this.min -= 0.5;
				this.max += 0.5;
			}
			this.single = single;

			if (!tickPositionsOption && !tickPositioner) {
				this.adjustTickAmount();
			}
		}
	},

	/**
	 * Handle startOnTick and endOnTick by either adapting to padding min/max or rounded min/max
	 */
	trimTicks: function (tickPositions, startOnTick, endOnTick) {
		var roundedMin = tickPositions[0],
			roundedMax = tickPositions[tickPositions.length - 1],
			minPointOffset = this.minPointOffset || 0;

		if (startOnTick) {
			this.min = roundedMin;
		} else if (this.min - minPointOffset > roundedMin) {
			tickPositions.shift();
		}

		if (endOnTick) {
			this.max = roundedMax;
		} else if (this.max + minPointOffset < roundedMax) {
			tickPositions.pop();
		}

		// If no tick are left, set one tick in the middle (#3195)
		if (tickPositions.length === 0 && defined(roundedMin)) {
			tickPositions.push((roundedMax + roundedMin) / 2);
		}
	},

	/**
	 * Check if there are multiple axes in the same pane
	 * @returns {Boolean} There are other axes
	 */
	alignToOthers: function () {
		var others = {}, // Whether there is another axis to pair with this one
			hasOther,
			options = this.options;

		if (this.chart.options.chart.alignTicks !== false && options.alignTicks !== false) {
			each(this.chart[this.coll], function (axis) {
				var otherOptions = axis.options,
					horiz = axis.horiz,
					key = [
						horiz ? otherOptions.left : otherOptions.top, 
						otherOptions.width,
						otherOptions.height, 
						otherOptions.pane
					].join(',');


				if (axis.series.length) { // #4442
					if (others[key]) {
						hasOther = true; // #4201
					} else {
						others[key] = 1;
					}
				}
			});
		}
		return hasOther;
	},

	/**
	 * Set the max ticks of either the x and y axis collection
	 */
	getTickAmount: function () {
		var options = this.options,
			tickAmount = options.tickAmount,
			tickPixelInterval = options.tickPixelInterval;

		if (!defined(options.tickInterval) && this.len < tickPixelInterval && !this.isRadial &&
				!this.isLog && options.startOnTick && options.endOnTick) {
			tickAmount = 2;
		}

		if (!tickAmount && this.alignToOthers()) {
			// Add 1 because 4 tick intervals require 5 ticks (including first and last)
			tickAmount = mathCeil(this.len / tickPixelInterval) + 1;
		}

		// For tick amounts of 2 and 3, compute five ticks and remove the intermediate ones. This
		// prevents the axis from adding ticks that are too far away from the data extremes.
		if (tickAmount < 4) {
			this.finalTickAmt = tickAmount;
			tickAmount = 5;
		}

		this.tickAmount = tickAmount;
	},

	/**
	 * When using multiple axes, adjust the number of ticks to match the highest
	 * number of ticks in that group
	 */
	adjustTickAmount: function () {
		var tickInterval = this.tickInterval,
			tickPositions = this.tickPositions,
			tickAmount = this.tickAmount,
			finalTickAmt = this.finalTickAmt,
			currentTickAmount = tickPositions && tickPositions.length,
			i,
			len;

		if (currentTickAmount < tickAmount) {
			while (tickPositions.length < tickAmount) {
				tickPositions.push(correctFloat(
					tickPositions[tickPositions.length - 1] + tickInterval
				));
			}
			this.transA *= (currentTickAmount - 1) / (tickAmount - 1);
			this.max = tickPositions[tickPositions.length - 1];

		// We have too many ticks, run second pass to try to reduce ticks
		} else if (currentTickAmount > tickAmount) {
			this.tickInterval *= 2;
			this.setTickPositions();
		}

		// The finalTickAmt property is set in getTickAmount
		if (defined(finalTickAmt)) {
			i = len = tickPositions.length;
			while (i--) {
				if (
					(finalTickAmt === 3 && i % 2 === 1) || // Remove every other tick
					(finalTickAmt <= 2 && i > 0 && i < len - 1) // Remove all but first and last
				) {
					tickPositions.splice(i, 1);
				}
			}
			this.finalTickAmt = UNDEFINED;
		}
	},

	/**
	 * Set the scale based on data min and max, user set min and max or options
	 *
	 */
	setScale: function () {
		var axis = this,
			isDirtyData,
			isDirtyAxisLength;

		axis.oldMin = axis.min;
		axis.oldMax = axis.max;
		axis.oldAxisLength = axis.len;

		// set the new axisLength
		axis.setAxisSize();
		//axisLength = horiz ? axisWidth : axisHeight;
		isDirtyAxisLength = axis.len !== axis.oldAxisLength;

		// is there new data?
		each(axis.series, function (series) {
			if (series.isDirtyData || series.isDirty ||
					series.xAxis.isDirty) { // when x axis is dirty, we need new data extremes for y as well
				isDirtyData = true;
			}
		});

		// do we really need to go through all this?
		if (isDirtyAxisLength || isDirtyData || axis.isLinked || axis.forceRedraw ||
			axis.userMin !== axis.oldUserMin || axis.userMax !== axis.oldUserMax || axis.alignToOthers()) {

			if (axis.resetStacks) {
				axis.resetStacks();
			}

			axis.forceRedraw = false;

			// get data extremes if needed
			axis.getSeriesExtremes();

			// get fixed positions based on tickInterval
			axis.setTickInterval();

			// record old values to decide whether a rescale is necessary later on (#540)
			axis.oldUserMin = axis.userMin;
			axis.oldUserMax = axis.userMax;

			// Mark as dirty if it is not already set to dirty and extremes have changed. #595.
			if (!axis.isDirty) {
				axis.isDirty = isDirtyAxisLength || axis.min !== axis.oldMin || axis.max !== axis.oldMax;
			}
		} else if (axis.cleanStacks) {
			axis.cleanStacks();
		}
	},

	/**
	 * Set the extremes and optionally redraw
	 * @param {Number} newMin
	 * @param {Number} newMax
	 * @param {Boolean} redraw
	 * @param {Boolean|Object} animation Whether to apply animation, and optionally animation
	 *    configuration
	 * @param {Object} eventArguments
	 *
	 */
	setExtremes: function (newMin, newMax, redraw, animation, eventArguments) {
		var axis = this,
			chart = axis.chart;

		redraw = pick(redraw, true); // defaults to true

		each(axis.series, function (serie) {
			delete serie.kdTree;
		});

		// Extend the arguments with min and max
		eventArguments = extend(eventArguments, {
			min: newMin,
			max: newMax
		});

		// Fire the event
		fireEvent(axis, 'setExtremes', eventArguments, function () { // the default event handler

			axis.userMin = newMin;
			axis.userMax = newMax;
			axis.eventArgs = eventArguments;

			if (redraw) {
				chart.redraw(animation);
			}
		});
	},

	/**
	 * Overridable method for zooming chart. Pulled out in a separate method to allow overriding
	 * in stock charts.
	 */
	zoom: function (newMin, newMax) {
		var dataMin = this.dataMin,
			dataMax = this.dataMax,
			options = this.options,
			min = mathMin(dataMin, pick(options.min, dataMin)),
			max = mathMax(dataMax, pick(options.max, dataMax));

		// Prevent pinch zooming out of range. Check for defined is for #1946. #1734.
		if (!this.allowZoomOutside) {
			if (defined(dataMin) && newMin <= min) {
				newMin = min;
			}
			if (defined(dataMax) && newMax >= max) {
				newMax = max;
			}
		}

		// In full view, displaying the reset zoom button is not required
		this.displayBtn = newMin !== UNDEFINED || newMax !== UNDEFINED;

		// Do it
		this.setExtremes(
			newMin,
			newMax,
			false,
			UNDEFINED,
			{ trigger: 'zoom' }
		);
		return true;
	},

	/**
	 * Update the axis metrics
	 */
	setAxisSize: function () {
		var chart = this.chart,
			options = this.options,
			offsetLeft = options.offsetLeft || 0,
			offsetRight = options.offsetRight || 0,
			horiz = this.horiz,
			width = pick(options.width, chart.plotWidth - offsetLeft + offsetRight),
			height = pick(options.height, chart.plotHeight),
			top = pick(options.top, chart.plotTop),
			left = pick(options.left, chart.plotLeft + offsetLeft),
			percentRegex = /%$/;

		// Check for percentage based input values
		if (percentRegex.test(height)) {
			height = parseFloat(height) / 100 * chart.plotHeight;
		}
		if (percentRegex.test(top)) {
			top = parseFloat(top) / 100 * chart.plotHeight + chart.plotTop;
		}

		// Expose basic values to use in Series object and navigator
		this.left = left;
		this.top = top;
		this.width = width;
		this.height = height;
		this.bottom = chart.chartHeight - height - top;
		this.right = chart.chartWidth - width - left;

		// Direction agnostic properties
		this.len = mathMax(horiz ? width : height, 0); // mathMax fixes #905
		this.pos = horiz ? left : top; // distance from SVG origin
	},

	/**
	 * Get the actual axis extremes
	 */
	getExtremes: function () {
		var axis = this,
			isLog = axis.isLog;

		return {
			min: isLog ? correctFloat(lin2log(axis.min)) : axis.min,
			max: isLog ? correctFloat(lin2log(axis.max)) : axis.max,
			dataMin: axis.dataMin,
			dataMax: axis.dataMax,
			userMin: axis.userMin,
			userMax: axis.userMax
		};
	},

	/**
	 * Get the zero plane either based on zero or on the min or max value.
	 * Used in bar and area plots
	 */
	getThreshold: function (threshold) {
		var axis = this,
			isLog = axis.isLog,
			realMin = isLog ? lin2log(axis.min) : axis.min,
			realMax = isLog ? lin2log(axis.max) : axis.max;

		// With a threshold of null, make the columns/areas rise from the top or bottom
		// depending on the value, assuming an actual threshold of 0 (#4233).
		if (threshold === null) {
			threshold = realMax < 0 ? realMax : realMin;
		} else if (realMin > threshold) {
			threshold = realMin;
		} else if (realMax < threshold) {
			threshold = realMax;
		}

		return axis.translate(threshold, 0, 1, 0, 1);
	},

	/**
	 * Compute auto alignment for the axis label based on which side the axis is on
	 * and the given rotation for the label
	 */
	autoLabelAlign: function (rotation) {
		var ret,
			angle = (pick(rotation, 0) - (this.side * 90) + 720) % 360;

		if (angle > 15 && angle < 165) {
			ret = 'right';
		} else if (angle > 195 && angle < 345) {
			ret = 'left';
		} else {
			ret = 'center';
		}
		return ret;
	},

	/**
	 * Prevent the ticks from getting so close we can't draw the labels. On a horizontal
	 * axis, this is handled by rotating the labels, removing ticks and adding ellipsis.
	 * On a vertical axis remove ticks and add ellipsis.
	 */
	unsquish: function () {
		var chart = this.chart,
			ticks = this.ticks,
			labelOptions = this.options.labels,
			horiz = this.horiz,
			tickInterval = this.tickInterval,
			newTickInterval = tickInterval,
			slotSize = this.len / (((this.categories ? 1 : 0) + this.max - this.min) / tickInterval),
			rotation,
			rotationOption = labelOptions.rotation,
			labelMetrics = chart.renderer.fontMetrics(labelOptions.style.fontSize, ticks[0] && ticks[0].label),
			step,
			bestScore = Number.MAX_VALUE,
			autoRotation,
			// Return the multiple of tickInterval that is needed to avoid collision
			getStep = function (spaceNeeded) {
				var step = spaceNeeded / (slotSize || 1);
				step = step > 1 ? mathCeil(step) : 1;
				return step * tickInterval;
			};

		if (horiz) {
			autoRotation = !labelOptions.staggerLines && !labelOptions.step && ( // #3971
				defined(rotationOption) ?
					[rotationOption] :
					slotSize < pick(labelOptions.autoRotationLimit, 80) && labelOptions.autoRotation
			);

			if (autoRotation) {

				// Loop over the given autoRotation options, and determine which gives the best score. The
				// best score is that with the lowest number of steps and a rotation closest to horizontal.
				each(autoRotation, function (rot) {
					var score;

					if (rot === rotationOption || (rot && rot >= -90 && rot <= 90)) { // #3891

						step = getStep(mathAbs(labelMetrics.h / mathSin(deg2rad * rot)));

						score = step + mathAbs(rot / 360);

						if (score < bestScore) {
							bestScore = score;
							rotation = rot;
							newTickInterval = step;
						}
					}
				});
			}

		} else if (!labelOptions.step) { // #4411
			newTickInterval = getStep(labelMetrics.h);
		}

		this.autoRotation = autoRotation;
		this.labelRotation = pick(rotation, rotationOption);

		return newTickInterval;
	},

	renderUnsquish: function () {
		var chart = this.chart,
			renderer = chart.renderer,
			tickPositions = this.tickPositions,
			ticks = this.ticks,
			labelOptions = this.options.labels,
			horiz = this.horiz,
			margin = chart.margin,
			slotCount = this.categories ? tickPositions.length : tickPositions.length - 1,
			slotWidth = this.slotWidth = (horiz && (labelOptions.step || 0) < 2 && !labelOptions.rotation && // #4415
				((this.staggerLines || 1) * chart.plotWidth) / slotCount) ||
				(!horiz && ((margin[3] && (margin[3] - chart.spacing[3])) || chart.chartWidth * 0.33)), // #1580, #1931,
			innerWidth = mathMax(1, mathRound(slotWidth - 2 * (labelOptions.padding || 5))),
			attr = {},
			labelMetrics = renderer.fontMetrics(labelOptions.style.fontSize, ticks[0] && ticks[0].label),
			textOverflowOption = labelOptions.style.textOverflow,
			css,
			labelLength = 0,
			label,
			i,
			pos;

		// Set rotation option unless it is "auto", like in gauges
		if (!isString(labelOptions.rotation)) {
			attr.rotation = labelOptions.rotation || 0; // #4443
		}

		// Handle auto rotation on horizontal axis
		if (this.autoRotation) {

			// Get the longest label length
			each(tickPositions, function (tick) {
				tick = ticks[tick];
				if (tick && tick.labelLength > labelLength) {
					labelLength = tick.labelLength;
				}
			});

			// Apply rotation only if the label is too wide for the slot, and
			// the label is wider than its height.
			if (labelLength > innerWidth && labelLength > labelMetrics.h) {
				attr.rotation = this.labelRotation;
			} else {
				this.labelRotation = 0;
			}

		// Handle word-wrap or ellipsis on vertical axis
		} else if (slotWidth) {
			// For word-wrap or ellipsis
			css = { width: innerWidth + PX };

			if (!textOverflowOption) {
				css.textOverflow = 'clip';

				// On vertical axis, only allow word wrap if there is room for more lines.
				i = tickPositions.length;
				while (!horiz && i--) {
					pos = tickPositions[i];
					label = ticks[pos].label;
					if (label) {
						// Reset ellipsis in order to get the correct bounding box (#4070)
						if (label.styles.textOverflow === 'ellipsis') {
							label.css({ textOverflow: 'clip' });
						}
						if (label.getBBox().height > this.len / tickPositions.length - (labelMetrics.h - labelMetrics.f) ||
								ticks[pos].labelLength > slotWidth) { // #4678
							label.specCss = { textOverflow: 'ellipsis' };
						}
					}
				}
			}
		}


		// Add ellipsis if the label length is significantly longer than ideal
		if (attr.rotation) {
			css = {
				width: (labelLength > chart.chartHeight * 0.5 ? chart.chartHeight * 0.33 : chart.chartHeight) + PX
			};
			if (!textOverflowOption) {
				css.textOverflow = 'ellipsis';
			}
		}

		// Set the explicit or automatic label alignment
		this.labelAlign = attr.align = labelOptions.align || this.autoLabelAlign(this.labelRotation);

		// Apply general and specific CSS
		each(tickPositions, function (pos) {
			var tick = ticks[pos],
				label = tick && tick.label;
			if (label) {
				label.attr(attr); // This needs to go before the CSS in old IE (#4502)
				if (css) {
					label.css(merge(css, label.specCss));
				}
				delete label.specCss;
				tick.rotation = attr.rotation;
			}
		});

		// Note: Why is this not part of getLabelPosition?
		this.tickRotCorr = renderer.rotCorr(labelMetrics.b, this.labelRotation || 0, this.side !== 0);
	},

	/**
	 * Return true if the axis has associated data
	 */
	hasData: function () {
		return this.hasVisibleSeries || (defined(this.min) && defined(this.max) && !!this.tickPositions);
	},

	/**
	 * Render the tick labels to a preliminary position to get their sizes
	 */
	getOffset: function () {
		var axis = this,
			chart = axis.chart,
			renderer = chart.renderer,
			options = axis.options,
			tickPositions = axis.tickPositions,
			ticks = axis.ticks,
			horiz = axis.horiz,
			side = axis.side,
			invertedSide = chart.inverted ? [1, 0, 3, 2][side] : side,
			hasData,
			showAxis,
			titleOffset = 0,
			titleOffsetOption,
			titleMargin = 0,
			axisTitleOptions = options.title,
			labelOptions = options.labels,
			labelOffset = 0, // reset
			labelOffsetPadded,
			opposite = axis.opposite,
			axisOffset = chart.axisOffset,
			clipOffset = chart.clipOffset,
			clip,
			directionFactor = [-1, 1, 1, -1][side],
			n,
			axisParent = axis.axisParent, // Used in color axis
			lineHeightCorrection;

		// For reuse in Axis.render
		hasData = axis.hasData();
		axis.showAxis = showAxis = hasData || pick(options.showEmpty, true);

		// Set/reset staggerLines
		axis.staggerLines = axis.horiz && labelOptions.staggerLines;

		// Create the axisGroup and gridGroup elements on first iteration
		if (!axis.axisGroup) {
			axis.gridGroup = renderer.g('grid')
				.attr({ zIndex: options.gridZIndex || 1 })
				.add(axisParent);
			axis.axisGroup = renderer.g('axis')
				.attr({ zIndex: options.zIndex || 2 })
				.add(axisParent);
			axis.labelGroup = renderer.g('axis-labels')
				.attr({ zIndex: labelOptions.zIndex || 7 })
				.addClass(PREFIX + axis.coll.toLowerCase() + '-labels')
				.add(axisParent);
		}

		if (hasData || axis.isLinked) {

			// Generate ticks
			each(tickPositions, function (pos) {
				if (!ticks[pos]) {
					ticks[pos] = new Tick(axis, pos);
				} else {
					ticks[pos].addLabel(); // update labels depending on tick interval
				}
			});

			axis.renderUnsquish();


			// Left side must be align: right and right side must have align: left for labels
			if (labelOptions.reserveSpace !== false && (side === 0 || side === 2 ||
					{ 1: 'left', 3: 'right' }[side] === axis.labelAlign || axis.labelAlign === 'center')) {
				each(tickPositions, function (pos) {

					// get the highest offset
					labelOffset = mathMax(
						ticks[pos].getLabelSize(),
						labelOffset
					);
				});
			}

			if (axis.staggerLines) {
				labelOffset *= axis.staggerLines;
				axis.labelOffset = labelOffset * (axis.opposite ? -1 : 1);
			}


		} else { // doesn't have data
			for (n in ticks) {
				ticks[n].destroy();
				delete ticks[n];
			}
		}

		if (axisTitleOptions && axisTitleOptions.text && axisTitleOptions.enabled !== false) {
			if (!axis.axisTitle) {
				axis.axisTitle = renderer.text(
					axisTitleOptions.text,
					0,
					0,
					axisTitleOptions.useHTML
				)
				.attr({
					zIndex: 7,
					rotation: axisTitleOptions.rotation || 0,
					align: 
						axisTitleOptions.textAlign ||
						{ 
							low: opposite ? 'right' : 'left',
							middle: 'center',
							high: opposite ? 'left' : 'right'
						}[axisTitleOptions.align]
				})
				.addClass(PREFIX + this.coll.toLowerCase() + '-title')
				.css(axisTitleOptions.style)
				.add(axis.axisGroup);
				axis.axisTitle.isNew = true;
			}

			if (showAxis) {
				titleOffset = axis.axisTitle.getBBox()[horiz ? 'height' : 'width'];
				titleOffsetOption = axisTitleOptions.offset;
				titleMargin = defined(titleOffsetOption) ? 0 : pick(axisTitleOptions.margin, horiz ? 5 : 10);
			}

			// hide or show the title depending on whether showEmpty is set
			axis.axisTitle[showAxis ? 'show' : 'hide'](true);
		}

		// handle automatic or user set offset
		axis.offset = directionFactor * pick(options.offset, axisOffset[side]);

		axis.tickRotCorr = axis.tickRotCorr || { x: 0, y: 0 }; // polar
		lineHeightCorrection = side === 2 ? axis.tickRotCorr.y : 0;
		labelOffsetPadded = Math.abs(labelOffset) + titleMargin +
			(labelOffset && (directionFactor * (horiz ? pick(labelOptions.y, axis.tickRotCorr.y + 8) : labelOptions.x) - lineHeightCorrection));
		axis.axisTitleMargin = pick(titleOffsetOption, labelOffsetPadded);

		axisOffset[side] = mathMax(
			axisOffset[side],
			axis.axisTitleMargin + titleOffset + directionFactor * axis.offset,
			labelOffsetPadded // #3027
		);

		// Decide the clipping needed to keep the graph inside the plot area and axis lines
		clip = options.offset ? 0 : mathFloor(options.lineWidth / 2) * 2; // #4308, #4371
		clipOffset[invertedSide] = mathMax(clipOffset[invertedSide], clip);
	},

	/**
	 * Get the path for the axis line
	 */
	getLinePath: function (lineWidth) {
		var chart = this.chart,
			opposite = this.opposite,
			offset = this.offset,
			horiz = this.horiz,
			lineLeft = this.left + (opposite ? this.width : 0) + offset,
			lineTop = chart.chartHeight - this.bottom - (opposite ? this.height : 0) + offset;

		if (opposite) {
			lineWidth *= -1; // crispify the other way - #1480, #1687
		}

		return chart.renderer
			.crispLine([
				M,
				horiz ?
					this.left :
					lineLeft,
				horiz ?
					lineTop :
					this.top,
				L,
				horiz ?
					chart.chartWidth - this.right :
					lineLeft,
				horiz ?
					lineTop :
					chart.chartHeight - this.bottom
			], lineWidth);
	},

	/**
	 * Position the title
	 */
	getTitlePosition: function () {
		// compute anchor points for each of the title align options
		var horiz = this.horiz,
			axisLeft = this.left,
			axisTop = this.top,
			axisLength = this.len,
			axisTitleOptions = this.options.title,
			margin = horiz ? axisLeft : axisTop,
			opposite = this.opposite,
			offset = this.offset,
			xOption = axisTitleOptions.x || 0,
			yOption = axisTitleOptions.y || 0,
			fontSize = pInt(axisTitleOptions.style.fontSize || 12),

			// the position in the length direction of the axis
			alongAxis = {
				low: margin + (horiz ? 0 : axisLength),
				middle: margin + axisLength / 2,
				high: margin + (horiz ? axisLength : 0)
			}[axisTitleOptions.align],

			// the position in the perpendicular direction of the axis
			offAxis = (horiz ? axisTop + this.height : axisLeft) +
				(horiz ? 1 : -1) * // horizontal axis reverses the margin
				(opposite ? -1 : 1) * // so does opposite axes
				this.axisTitleMargin +
				(this.side === 2 ? fontSize : 0);

		return {
			x: horiz ?
				alongAxis + xOption :
				offAxis + (opposite ? this.width : 0) + offset + xOption,
			y: horiz ?
				offAxis + yOption - (opposite ? this.height : 0) + offset :
				alongAxis + yOption
		};
	},

	/**
	 * Render the axis
	 */
	render: function () {
		var axis = this,
			chart = axis.chart,
			renderer = chart.renderer,
			options = axis.options,
			isLog = axis.isLog,
			isLinked = axis.isLinked,
			tickPositions = axis.tickPositions,
			axisTitle = axis.axisTitle,
			ticks = axis.ticks,
			minorTicks = axis.minorTicks,
			alternateBands = axis.alternateBands,
			stackLabelOptions = options.stackLabels,
			alternateGridColor = options.alternateGridColor,
			tickmarkOffset = axis.tickmarkOffset,
			lineWidth = options.lineWidth,
			linePath,
			hasRendered = chart.hasRendered,
			slideInTicks = hasRendered && defined(axis.oldMin) && !isNaN(axis.oldMin),
			showAxis = axis.showAxis,
			globalAnimation = renderer.globalAnimation,
			from,
			to;

		// Reset
		axis.labelEdge.length = 0;
		//axis.justifyToPlot = overflow === 'justify';
		axis.overlap = false;

		// Mark all elements inActive before we go over and mark the active ones
		each([ticks, minorTicks, alternateBands], function (coll) {
			var pos;
			for (pos in coll) {
				coll[pos].isActive = false;
			}
		});

		// If the series has data draw the ticks. Else only the line and title
		if (axis.hasData() || isLinked) {

			// minor ticks
			if (axis.minorTickInterval && !axis.categories) {
				each(axis.getMinorTickPositions(), function (pos) {
					if (!minorTicks[pos]) {
						minorTicks[pos] = new Tick(axis, pos, 'minor');
					}

					// render new ticks in old position
					if (slideInTicks && minorTicks[pos].isNew) {
						minorTicks[pos].render(null, true);
					}

					minorTicks[pos].render(null, false, 1);
				});
			}

			// Major ticks. Pull out the first item and render it last so that
			// we can get the position of the neighbour label. #808.
			if (tickPositions.length) { // #1300
				each(tickPositions, function (pos, i) {

					// linked axes need an extra check to find out if
					if (!isLinked || (pos >= axis.min && pos <= axis.max)) {

						if (!ticks[pos]) {
							ticks[pos] = new Tick(axis, pos);
						}

						// render new ticks in old position
						if (slideInTicks && ticks[pos].isNew) {
							ticks[pos].render(i, true, 0.1);
						}

						ticks[pos].render(i);
					}

				});
				// In a categorized axis, the tick marks are displayed between labels. So
				// we need to add a tick mark and grid line at the left edge of the X axis.
				if (tickmarkOffset && (axis.min === 0 || axis.single)) {
					if (!ticks[-1]) {
						ticks[-1] = new Tick(axis, -1, null, true);
					}
					ticks[-1].render(-1);
				}

			}

			// alternate grid color
			if (alternateGridColor) {
				each(tickPositions, function (pos, i) {
					to = tickPositions[i + 1] !== UNDEFINED ? tickPositions[i + 1] + tickmarkOffset : axis.max - tickmarkOffset; 
					if (i % 2 === 0 && pos < axis.max && to <= axis.max + (chart.polar ? -tickmarkOffset : tickmarkOffset)) { // #2248, #4660
						if (!alternateBands[pos]) {
							alternateBands[pos] = new Highcharts.PlotLineOrBand(axis);
						}
						from = pos + tickmarkOffset; // #949
						alternateBands[pos].options = {
							from: isLog ? lin2log(from) : from,
							to: isLog ? lin2log(to) : to,
							color: alternateGridColor
						};
						alternateBands[pos].render();
						alternateBands[pos].isActive = true;
					}
				});
			}

			// custom plot lines and bands
			if (!axis._addedPlotLB) { // only first time
				each((options.plotLines || []).concat(options.plotBands || []), function (plotLineOptions) {
					axis.addPlotBandOrLine(plotLineOptions);
				});
				axis._addedPlotLB = true;
			}

		} // end if hasData

		// Remove inactive ticks
		each([ticks, minorTicks, alternateBands], function (coll) {
			var pos,
				i,
				forDestruction = [],
				delay = globalAnimation ? globalAnimation.duration || 500 : 0,
				destroyInactiveItems = function () {
					i = forDestruction.length;
					while (i--) {
						// When resizing rapidly, the same items may be destroyed in different timeouts,
						// or the may be reactivated
						if (coll[forDestruction[i]] && !coll[forDestruction[i]].isActive) {
							coll[forDestruction[i]].destroy();
							delete coll[forDestruction[i]];
						}
					}

				};

			for (pos in coll) {

				if (!coll[pos].isActive) {
					// Render to zero opacity
					coll[pos].render(pos, false, 0);
					coll[pos].isActive = false;
					forDestruction.push(pos);
				}
			}

			// When the objects are finished fading out, destroy them
			syncTimeout(
				destroyInactiveItems, 
				coll === alternateBands || !chart.hasRendered || !delay ? 0 : delay
			);
		});

		// Static items. As the axis group is cleared on subsequent calls
		// to render, these items are added outside the group.
		// axis line
		if (lineWidth) {
			linePath = axis.getLinePath(lineWidth);
			if (!axis.axisLine) {
				axis.axisLine = renderer.path(linePath)
					.attr({
						stroke: options.lineColor,
						'stroke-width': lineWidth,
						zIndex: 7
					})
					.add(axis.axisGroup);
			} else {
				axis.axisLine.animate({ d: linePath });
			}

			// show or hide the line depending on options.showEmpty
			axis.axisLine[showAxis ? 'show' : 'hide'](true);
		}

		if (axisTitle && showAxis) {

			axisTitle[axisTitle.isNew ? 'attr' : 'animate'](
				axis.getTitlePosition()
			);
			axisTitle.isNew = false;
		}

		// Stacked totals:
		if (stackLabelOptions && stackLabelOptions.enabled) {
			axis.renderStackTotals();
		}
		// End stacked totals

		axis.isDirty = false;
	},

	/**
	 * Redraw the axis to reflect changes in the data or axis extremes
	 */
	redraw: function () {

		if (this.visible) {
			// render the axis
			this.render();

			// move plot lines and bands
			each(this.plotLinesAndBands, function (plotLine) {
				plotLine.render();
			});
		}

		// mark associated series as dirty and ready for redraw
		each(this.series, function (series) {
			series.isDirty = true;
		});

	},

	/**
	 * Destroys an Axis instance.
	 */
	destroy: function (keepEvents) {
		var axis = this,
			stacks = axis.stacks,
			stackKey,
			plotLinesAndBands = axis.plotLinesAndBands,
			i;

		// Remove the events
		if (!keepEvents) {
			removeEvent(axis);
		}

		// Destroy each stack total
		for (stackKey in stacks) {
			destroyObjectProperties(stacks[stackKey]);

			stacks[stackKey] = null;
		}

		// Destroy collections
		each([axis.ticks, axis.minorTicks, axis.alternateBands], function (coll) {
			destroyObjectProperties(coll);
		});
		i = plotLinesAndBands.length;
		while (i--) { // #1975
			plotLinesAndBands[i].destroy();
		}

		// Destroy local variables
		each(['stackTotalGroup', 'axisLine', 'axisTitle', 'axisGroup', 'cross', 'gridGroup', 'labelGroup'], function (prop) {
			if (axis[prop]) {
				axis[prop] = axis[prop].destroy();
			}
		});

		// Destroy crosshair
		if (this.cross) {
			this.cross.destroy();
		}
	},

	/**
	 * Draw the crosshair
	 * 
	 * @param  {Object} e The event arguments from the modified pointer event
	 * @param  {Object} point The Point object
	 */
	drawCrosshair: function (e, point) {

		var path,
			options = this.crosshair,
			pos,
			attribs,
			categorized,
			strokeWidth;

		if (
			// Disabled in options
			!this.crosshair ||
			// Snap
			((defined(point) || !pick(options.snap, true)) === false) ||
			// Not on this axis (#4095, #2888)
			(point && point.series && point.series[this.coll] !== this)
		) {
			this.hideCrosshair();

		} else {

			// Get the path
			if (!pick(options.snap, true)) {
				pos = (this.horiz ? e.chartX - this.pos : this.len - e.chartY + this.pos);
			} else if (defined(point)) {
				pos = this.isXAxis ? point.plotX : this.len - point.plotY; // #3834
			}

			if (this.isRadial) {
				path = this.getPlotLinePath(this.isXAxis ? point.x : pick(point.stackY, point.y)) || null; // #3189
			} else {
				path = this.getPlotLinePath(null, null, null, null, pos) || null; // #3189
			}

			if (path === null) {
				this.hideCrosshair();
				return;
			}

			categorized = this.categories && !this.isRadial;
			strokeWidth = pick(options.width, (categorized ? this.transA : 1));

			// Draw the cross
			if (this.cross) {
				this.cross
					.attr({
						d: path,
						visibility: 'visible',
						'stroke-width': strokeWidth // #4737
					});
			} else {
				attribs = {
					'stroke-width': strokeWidth,
					stroke: options.color || (categorized ? 'rgba(155,200,255,0.2)' : '#C0C0C0'),
					zIndex: pick(options.zIndex, 2)
				};
				if (options.dashStyle) {
					attribs.dashstyle = options.dashStyle;
				}
				this.cross = this.chart.renderer.path(path).attr(attribs).add();
			}

		}

	},

	/**
	 *	Hide the crosshair.
	 */
	hideCrosshair: function () {
		if (this.cross) {
			this.cross.hide();
		}
	}
}; // end Axis

extend(Axis.prototype, AxisPlotLineOrBandExtension);
