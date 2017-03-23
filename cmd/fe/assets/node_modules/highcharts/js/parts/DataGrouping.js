/* ****************************************************************************
 * Start data grouping module												 *
 ******************************************************************************/
var DATA_GROUPING = 'dataGrouping',
	seriesProto = Series.prototype,
	baseProcessData = seriesProto.processData,
	baseGeneratePoints = seriesProto.generatePoints,
	baseDestroy = seriesProto.destroy,

	commonOptions = {
		approximation: 'average', // average, open, high, low, close, sum
		//enabled: null, // (true for stock charts, false for basic),
		//forced: undefined,
		groupPixelWidth: 2,
		// the first one is the point or start value, the second is the start value if we're dealing with range,
		// the third one is the end value if dealing with a range
		dateTimeLabelFormats: {
			millisecond: ['%A, %b %e, %H:%M:%S.%L', '%A, %b %e, %H:%M:%S.%L', '-%H:%M:%S.%L'],
			second: ['%A, %b %e, %H:%M:%S', '%A, %b %e, %H:%M:%S', '-%H:%M:%S'],
			minute: ['%A, %b %e, %H:%M', '%A, %b %e, %H:%M', '-%H:%M'],
			hour: ['%A, %b %e, %H:%M', '%A, %b %e, %H:%M', '-%H:%M'],
			day: ['%A, %b %e, %Y', '%A, %b %e', '-%A, %b %e, %Y'],
			week: ['Week from %A, %b %e, %Y', '%A, %b %e', '-%A, %b %e, %Y'],
			month: ['%B %Y', '%B', '-%B %Y'],
			year: ['%Y', '%Y', '-%Y']
		}
		// smoothed = false, // enable this for navigator series only
	},

	specificOptions = { // extends common options
		line: {},
		spline: {},
		area: {},
		areaspline: {},
		column: {
			approximation: 'sum',
			groupPixelWidth: 10
		},
		arearange: {
			approximation: 'range'
		},
		areasplinerange: {
			approximation: 'range'
		},
		columnrange: {
			approximation: 'range',
			groupPixelWidth: 10
		},
		candlestick: {
			approximation: 'ohlc',
			groupPixelWidth: 10
		},
		ohlc: {
			approximation: 'ohlc',
			groupPixelWidth: 5
		}
	},

	// units are defined in a separate array to allow complete overriding in case of a user option
	defaultDataGroupingUnits = [[
			'millisecond', // unit name
			[1, 2, 5, 10, 20, 25, 50, 100, 200, 500] // allowed multiples
		], [
			'second',
			[1, 2, 5, 10, 15, 30]
		], [
			'minute',
			[1, 2, 5, 10, 15, 30]
		], [
			'hour',
			[1, 2, 3, 4, 6, 8, 12]
		], [
			'day',
			[1]
		], [
			'week',
			[1]
		], [
			'month',
			[1, 3, 6]
		], [
			'year',
			null
		]
	],


	/**
	 * Define the available approximation types. The data grouping approximations takes an array
	 * or numbers as the first parameter. In case of ohlc, four arrays are sent in as four parameters.
	 * Each array consists only of numbers. In case null values belong to the group, the property
	 * .hasNulls will be set to true on the array.
	 */
	approximations = {
		sum: function (arr) {
			var len = arr.length,
				ret;

			// 1. it consists of nulls exclusively
			if (!len && arr.hasNulls) {
				ret = null;
			// 2. it has a length and real values
			} else if (len) {
				ret = 0;
				while (len--) {
					ret += arr[len];
				}
			}
			// 3. it has zero length, so just return undefined
			// => doNothing()

			return ret;
		},
		average: function (arr) {
			var len = arr.length,
				ret = approximations.sum(arr);

			// If we have a number, return it divided by the length. If not, return
			// null or undefined based on what the sum method finds.
			if (typeof ret === 'number' && len) {
				ret = ret / len;
			}

			return ret;
		},
		open: function (arr) {
			return arr.length ? arr[0] : (arr.hasNulls ? null : UNDEFINED);
		},
		high: function (arr) {
			return arr.length ? arrayMax(arr) : (arr.hasNulls ? null : UNDEFINED);
		},
		low: function (arr) {
			return arr.length ? arrayMin(arr) : (arr.hasNulls ? null : UNDEFINED);
		},
		close: function (arr) {
			return arr.length ? arr[arr.length - 1] : (arr.hasNulls ? null : UNDEFINED);
		},
		// ohlc and range are special cases where a multidimensional array is input and an array is output
		ohlc: function (open, high, low, close) {
			open = approximations.open(open);
			high = approximations.high(high);
			low = approximations.low(low);
			close = approximations.close(close);

			if (typeof open === 'number' || typeof high === 'number' || typeof low === 'number' || typeof close === 'number') {
				return [open, high, low, close];
			}
			// else, return is undefined
		},
		range: function (low, high) {
			low = approximations.low(low);
			high = approximations.high(high);

			if (typeof low === 'number' || typeof high === 'number') {
				return [low, high];
			}
			// else, return is undefined
		}
	};


/**
 * Takes parallel arrays of x and y data and groups the data into intervals defined by groupPositions, a collection
 * of starting x values for each group.
 */
seriesProto.groupData = function (xData, yData, groupPositions, approximation) {
	var series = this,
		data = series.data,
		dataOptions = series.options.data,
		groupedXData = [],
		groupedYData = [],
		dataLength = xData.length,
		pointX,
		pointY,
		groupedY,
		handleYData = !!yData, // when grouping the fake extended axis for panning, we don't need to consider y
		values = [[], [], [], []],
		approximationFn = typeof approximation === 'function' ? approximation : approximations[approximation],
		pointArrayMap = series.pointArrayMap,
		pointArrayMapLength = pointArrayMap && pointArrayMap.length,
		i;

	// Start with the first point within the X axis range (#2696)
	for (i = 0; i <= dataLength; i++) {
		if (xData[i] >= groupPositions[0]) {
			break;
		}
	}

	for (i; i <= dataLength; i++) {

		// when a new group is entered, summarize and initiate the previous group
		while ((groupPositions[1] !== UNDEFINED && xData[i] >= groupPositions[1]) ||
				i === dataLength) { // get the last group

			// get group x and y
			pointX = groupPositions.shift();
			groupedY = approximationFn.apply(0, values);

			// push the grouped data
			if (groupedY !== UNDEFINED) {
				groupedXData.push(pointX);
				groupedYData.push(groupedY);
			}

			// reset the aggregate arrays
			values[0] = [];
			values[1] = [];
			values[2] = [];
			values[3] = [];

			// don't loop beyond the last group
			if (i === dataLength) {
				break;
			}
		}

		// break out
		if (i === dataLength) {
			break;
		}

		// for each raw data point, push it to an array that contains all values for this specific group
		if (pointArrayMap) {

			var index = series.cropStart + i,
				point = (data && data[index]) || series.pointClass.prototype.applyOptions.apply({ series: series }, [dataOptions[index]]),
				j,
				val;

			for (j = 0; j < pointArrayMapLength; j++) {
				val = point[pointArrayMap[j]];
				if (typeof val === 'number') {
					values[j].push(val);
				} else if (val === null) {
					values[j].hasNulls = true;
				}
			}

		} else {
			pointY = handleYData ? yData[i] : null;

			if (typeof pointY === 'number') {
				values[0].push(pointY);
			} else if (pointY === null) {
				values[0].hasNulls = true;
			}
		}
	}

	return [groupedXData, groupedYData];
};

/**
 * Extend the basic processData method, that crops the data to the current zoom
 * range, with data grouping logic.
 */
seriesProto.processData = function () {
	var series = this,
		chart = series.chart,
		options = series.options,
		dataGroupingOptions = options[DATA_GROUPING],
		groupingEnabled = series.allowDG !== false && dataGroupingOptions && pick(dataGroupingOptions.enabled, chart.options._stock),
		hasGroupedData,
		skip;

	// run base method
	series.forceCrop = groupingEnabled; // #334
	series.groupPixelWidth = null; // #2110
	series.hasProcessed = true; // #2692

	// skip if processData returns false or if grouping is disabled (in that order)
	skip = baseProcessData.apply(series, arguments) === false || !groupingEnabled;
	if (!skip) {
		series.destroyGroupedData();

		var i,
			processedXData = series.processedXData,
			processedYData = series.processedYData,
			plotSizeX = chart.plotSizeX,
			xAxis = series.xAxis,
			ordinal = xAxis.options.ordinal,
			groupPixelWidth = series.groupPixelWidth = xAxis.getGroupPixelWidth && xAxis.getGroupPixelWidth();

		// Execute grouping if the amount of points is greater than the limit defined in groupPixelWidth
		if (groupPixelWidth) {
			hasGroupedData = true;

			series.points = null; // force recreation of point instances in series.translate

			var extremes = xAxis.getExtremes(),
				xMin = extremes.min,
				xMax = extremes.max,
				groupIntervalFactor = (ordinal && xAxis.getGroupIntervalFactor(xMin, xMax, series)) || 1,
				interval = (groupPixelWidth * (xMax - xMin) / plotSizeX) * groupIntervalFactor,
				groupPositions = xAxis.getTimeTicks(
					xAxis.normalizeTimeTickInterval(interval, dataGroupingOptions.units || defaultDataGroupingUnits),
					xMin,
					xMax,
					xAxis.options.startOfWeek,
					processedXData,
					series.closestPointRange
				),
				groupedXandY = seriesProto.groupData.apply(series, [processedXData, processedYData, groupPositions, dataGroupingOptions.approximation]),
				groupedXData = groupedXandY[0],
				groupedYData = groupedXandY[1];

			// prevent the smoothed data to spill out left and right, and make
			// sure data is not shifted to the left
			if (dataGroupingOptions.smoothed) {
				i = groupedXData.length - 1;
				groupedXData[i] = Math.min(groupedXData[i], xMax);
				while (i-- && i > 0) {
					groupedXData[i] += interval / 2;
				}
				groupedXData[0] = Math.max(groupedXData[0], xMin);
			}

			// record what data grouping values were used
			series.currentDataGrouping = groupPositions.info;
			series.closestPointRange = groupPositions.info.totalRange;

			// Make sure the X axis extends to show the first group (#2533)
			if (defined(groupedXData[0]) && groupedXData[0] < xAxis.dataMin) {
				if (xAxis.min === xAxis.dataMin) {
					xAxis.min = groupedXData[0];
				}
				xAxis.dataMin = groupedXData[0];
			}

			// set series props
			series.processedXData = groupedXData;
			series.processedYData = groupedYData;
		} else {
			series.currentDataGrouping = null;
		}
		series.hasGroupedData = hasGroupedData;
	}
};

/**
 * Destroy the grouped data points. #622, #740
 */
seriesProto.destroyGroupedData = function () {

	var groupedData = this.groupedData;

	// clear previous groups
	each(groupedData || [], function (point, i) {
		if (point) {
			groupedData[i] = point.destroy ? point.destroy() : null;
		}
	});
	this.groupedData = null;
};

/**
 * Override the generatePoints method by adding a reference to grouped data
 */
seriesProto.generatePoints = function () {

	baseGeneratePoints.apply(this);

	// record grouped data in order to let it be destroyed the next time processData runs
	this.destroyGroupedData(); // #622
	this.groupedData = this.hasGroupedData ? this.points : null;
};

/**
 * Extend the original method, make the tooltip's header reflect the grouped range
 */
wrap(Tooltip.prototype, 'tooltipFooterHeaderFormatter', function (proceed, point, isFooter) {
	var tooltip = this,
		series = point.series,
		options = series.options,
		tooltipOptions = series.tooltipOptions,
		dataGroupingOptions = options.dataGrouping,
		xDateFormat = tooltipOptions.xDateFormat,
		xDateFormatEnd,
		xAxis = series.xAxis,
		currentDataGrouping,
		dateTimeLabelFormats,
		labelFormats,
		formattedKey;

	// apply only to grouped series
	if (xAxis && xAxis.options.type === 'datetime' && dataGroupingOptions && isNumber(point.key)) {

		// set variables
		currentDataGrouping = series.currentDataGrouping;
		dateTimeLabelFormats = dataGroupingOptions.dateTimeLabelFormats;

		// if we have grouped data, use the grouping information to get the right format
		if (currentDataGrouping) {
			labelFormats = dateTimeLabelFormats[currentDataGrouping.unitName];
			if (currentDataGrouping.count === 1) {
				xDateFormat = labelFormats[0];
			} else {
				xDateFormat = labelFormats[1];
				xDateFormatEnd = labelFormats[2];
			}
		// if not grouped, and we don't have set the xDateFormat option, get the best fit,
		// so if the least distance between points is one minute, show it, but if the
		// least distance is one day, skip hours and minutes etc.
		} else if (!xDateFormat && dateTimeLabelFormats) {
			xDateFormat = tooltip.getXDateFormat(point, tooltipOptions, xAxis);
		}

		// now format the key
		formattedKey = dateFormat(xDateFormat, point.key);
		if (xDateFormatEnd) {
			formattedKey += dateFormat(xDateFormatEnd, point.key + currentDataGrouping.totalRange - 1);
		}

		// return the replaced format
		return format(tooltipOptions[(isFooter ? 'footer' : 'header') + 'Format'], {
			point: extend(point, { key: formattedKey }),
			series: series
		});
	
	}

	// else, fall back to the regular formatter
	return proceed.call(tooltip, point, isFooter);
});

/**
 * Extend the series destroyer
 */
seriesProto.destroy = function () {
	var series = this,
		groupedData = series.groupedData || [],
		i = groupedData.length;

	while (i--) {
		if (groupedData[i]) {
			groupedData[i].destroy();
		}
	}
	baseDestroy.apply(series);
};


// Handle default options for data grouping. This must be set at runtime because some series types are
// defined after this.
wrap(seriesProto, 'setOptions', function (proceed, itemOptions) {

	var options = proceed.call(this, itemOptions),
		type = this.type,
		plotOptions = this.chart.options.plotOptions,
		defaultOptions = defaultPlotOptions[type].dataGrouping;

	if (specificOptions[type]) { // #1284
		if (!defaultOptions) {
			defaultOptions = merge(commonOptions, specificOptions[type]);
		}

		options.dataGrouping = merge(
			defaultOptions,
			plotOptions.series && plotOptions.series.dataGrouping, // #1228
			plotOptions[type].dataGrouping, // Set by the StockChart constructor
			itemOptions.dataGrouping
		);
	}

	if (this.chart.options._stock) {
		this.requireSorting = true;
	}

	return options;
});


/**
 * When resetting the scale reset the hasProccessed flag to avoid taking previous data grouping
 * of neighbour series into accound when determining group pixel width (#2692).
 */
wrap(Axis.prototype, 'setScale', function (proceed) {
	proceed.call(this);
	each(this.series, function (series) {
		series.hasProcessed = false;
	});
});

/**
 * Get the data grouping pixel width based on the greatest defined individual width
 * of the axis' series, and if whether one of the axes need grouping.
 */
Axis.prototype.getGroupPixelWidth = function () {

	var series = this.series,
		len = series.length,
		i,
		groupPixelWidth = 0,
		doGrouping = false,
		dataLength,
		dgOptions;

	// If multiple series are compared on the same x axis, give them the same
	// group pixel width (#334)
	i = len;
	while (i--) {
		dgOptions = series[i].options.dataGrouping;
		if (dgOptions) {
			groupPixelWidth = mathMax(groupPixelWidth, dgOptions.groupPixelWidth);

		}
	}

	// If one of the series needs grouping, apply it to all (#1634)
	i = len;
	while (i--) {
		dgOptions = series[i].options.dataGrouping;

		if (dgOptions && series[i].hasProcessed) { // #2692

			dataLength = (series[i].processedXData || series[i].data).length;

			// Execute grouping if the amount of points is greater than the limit defined in groupPixelWidth
			if (series[i].groupPixelWidth || dataLength > (this.chart.plotSizeX / groupPixelWidth) || (dataLength && dgOptions.forced)) {
				doGrouping = true;
			}
		}
	}

	return doGrouping ? groupPixelWidth : 0;
};

/**
 * Force data grouping on all the axis' series.
 */
Axis.prototype.setDataGrouping = function (dataGrouping, redraw) {
	var i;

	redraw = pick(redraw, true);

	if (!dataGrouping) {
		dataGrouping = {
			forced: false,
			units: null
		};
	}

	// Axis is instantiated, update all series
	if (this instanceof Axis) {
		i = this.series.length;
		while (i--) {
			this.series[i].update({
				dataGrouping: dataGrouping
			}, false);
		}

	// Axis not yet instanciated, alter series options
	} else {
		each(this.chart.options.series, function (seriesOptions) {
			seriesOptions.dataGrouping = dataGrouping;
		}, false);
	}

	if (redraw) {
		this.chart.redraw();
	}
};



/* ****************************************************************************
 * End data grouping module												   *
 ******************************************************************************/