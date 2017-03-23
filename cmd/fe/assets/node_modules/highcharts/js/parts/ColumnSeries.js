/**
 * Set the default options for column
 */
defaultPlotOptions.column = merge(defaultSeriesOptions, {
	borderColor: '#FFFFFF',
	//borderWidth: 1,
	borderRadius: 0,
	//colorByPoint: undefined,
	groupPadding: 0.2,
	//grouping: true,
	marker: null, // point options are specified in the base options
	pointPadding: 0.1,
	//pointWidth: null,
	minPointLength: 0,
	cropThreshold: 50, // when there are more points, they will not animate out of the chart on xAxis.setExtremes
	pointRange: null, // null means auto, meaning 1 in a categorized axis and least distance between points if not categories
	states: {
		hover: {
			brightness: 0.1,
			shadow: false,
			halo: false
		},
		select: {
			color: '#C0C0C0',
			borderColor: '#000000',
			shadow: false
		}
	},
	dataLabels: {
		align: null, // auto
		verticalAlign: null, // auto
		y: null
	},
	softThreshold: false,
	startFromThreshold: true, // false doesn't work well: http://jsfiddle.net/highcharts/hz8fopan/14/
	stickyTracking: false,
	tooltip: {
		distance: 6
	},
	threshold: 0
});

/**
 * ColumnSeries object
 */
var ColumnSeries = extendClass(Series, {
	type: 'column',
	pointAttrToOptions: { // mapping between SVG attributes and the corresponding options
		stroke: 'borderColor',
		fill: 'color',
		r: 'borderRadius'
	},
	cropShoulder: 0,
	directTouch: true, // When tooltip is not shared, this series (and derivatives) requires direct touch/hover. KD-tree does not apply.
	trackerGroups: ['group', 'dataLabelsGroup'],
	negStacks: true, // use separate negative stacks, unlike area stacks where a negative
		// point is substracted from previous (#1910)

	/**
	 * Initialize the series
	 */
	init: function () {
		Series.prototype.init.apply(this, arguments);

		var series = this,
			chart = series.chart;

		// if the series is added dynamically, force redraw of other
		// series affected by a new column
		if (chart.hasRendered) {
			each(chart.series, function (otherSeries) {
				if (otherSeries.type === series.type) {
					otherSeries.isDirty = true;
				}
			});
		}
	},

	/**
	 * Return the width and x offset of the columns adjusted for grouping, groupPadding, pointPadding,
	 * pointWidth etc.
	 */
	getColumnMetrics: function () {

		var series = this,
			options = series.options,
			xAxis = series.xAxis,
			yAxis = series.yAxis,
			reversedXAxis = xAxis.reversed,
			stackKey,
			stackGroups = {},
			columnIndex,
			columnCount = 0;

		// Get the total number of column type series.
		// This is called on every series. Consider moving this logic to a
		// chart.orderStacks() function and call it on init, addSeries and removeSeries
		if (options.grouping === false) {
			columnCount = 1;
		} else {
			each(series.chart.series, function (otherSeries) {
				var otherOptions = otherSeries.options,
					otherYAxis = otherSeries.yAxis;
				if (otherSeries.type === series.type && otherSeries.visible &&
						yAxis.len === otherYAxis.len && yAxis.pos === otherYAxis.pos) {  // #642, #2086
					if (otherOptions.stacking) {
						stackKey = otherSeries.stackKey;
						if (stackGroups[stackKey] === UNDEFINED) {
							stackGroups[stackKey] = columnCount++;
						}
						columnIndex = stackGroups[stackKey];
					} else if (otherOptions.grouping !== false) { // #1162
						columnIndex = columnCount++;
					}
					otherSeries.columnIndex = columnIndex;
				}
			});
		}

		var categoryWidth = mathMin(
				mathAbs(xAxis.transA) * (xAxis.ordinalSlope || options.pointRange || xAxis.closestPointRange || xAxis.tickInterval || 1), // #2610
				xAxis.len // #1535
			),
			groupPadding = categoryWidth * options.groupPadding,
			groupWidth = categoryWidth - 2 * groupPadding,
			pointOffsetWidth = groupWidth / columnCount,
			pointWidth = mathMin(
				options.maxPointWidth || xAxis.len,
				pick(options.pointWidth, pointOffsetWidth * (1 - 2 * options.pointPadding))
			),
			pointPadding = (pointOffsetWidth - pointWidth) / 2,
			colIndex = (series.columnIndex || 0) + (reversedXAxis ? 1 : 0), // #1251, #3737
			pointXOffset = pointPadding + (groupPadding + colIndex *
				pointOffsetWidth - (categoryWidth / 2)) *
				(reversedXAxis ? -1 : 1);

		// Save it for reading in linked series (Error bars particularly)
		series.columnMetrics = {
			width: pointWidth,
			offset: pointXOffset
		};
		return series.columnMetrics;

	},

	/**
	 * Make the columns crisp. The edges are rounded to the nearest full pixel.
	 */
	crispCol: function (x, y, w, h) {
		var chart = this.chart,
			borderWidth = this.borderWidth,
			xCrisp = -(borderWidth % 2 ? 0.5 : 0),
			yCrisp = borderWidth % 2 ? 0.5 : 1,
			right,
			bottom,
			fromTop;

		if (chart.inverted && chart.renderer.isVML) {
			yCrisp += 1;
		}

		// Horizontal. We need to first compute the exact right edge, then round it
		// and compute the width from there.
		right = Math.round(x + w) + xCrisp;
		x = Math.round(x) + xCrisp;
		w = right - x;

		// Vertical
		bottom = Math.round(y + h) + yCrisp;
		fromTop = mathAbs(y) <= 0.5 && bottom > 0.5; // #4504, #4656
		y = Math.round(y) + yCrisp;
		h = bottom - y;

		// Top edges are exceptions
		if (fromTop) {
			y -= 1;
			h += 1;
		}

		return {
			x: x,
			y: y,
			width: w,
			height: h
		};
	},

	/**
	 * Translate each point to the plot area coordinate system and find shape positions
	 */
	translate: function () {
		var series = this,
			chart = series.chart,
			options = series.options,
			borderWidth = series.borderWidth = pick(
				options.borderWidth,
				series.closestPointRange * series.xAxis.transA < 2 ? 0 : 1 // #3635
			),
			yAxis = series.yAxis,
			threshold = options.threshold,
			translatedThreshold = series.translatedThreshold = yAxis.getThreshold(threshold),
			minPointLength = pick(options.minPointLength, 5),
			metrics = series.getColumnMetrics(),
			pointWidth = metrics.width,
			seriesBarW = series.barW = mathMax(pointWidth, 1 + 2 * borderWidth), // postprocessed for border width
			pointXOffset = series.pointXOffset = metrics.offset;

		if (chart.inverted) {
			translatedThreshold -= 0.5; // #3355
		}

		// When the pointPadding is 0, we want the columns to be packed tightly, so we allow individual
		// columns to have individual sizes. When pointPadding is greater, we strive for equal-width
		// columns (#2694).
		if (options.pointPadding) {
			seriesBarW = mathCeil(seriesBarW);
		}

		Series.prototype.translate.apply(series);

		// Record the new values
		each(series.points, function (point) {
			var yBottom = mathMin(pick(point.yBottom, translatedThreshold), 9e4), // #3575
				safeDistance = 999 + mathAbs(yBottom),
				plotY = mathMin(mathMax(-safeDistance, point.plotY), yAxis.len + safeDistance), // Don't draw too far outside plot area (#1303, #2241, #4264)
				barX = point.plotX + pointXOffset,
				barW = seriesBarW,
				barY = mathMin(plotY, yBottom),
				up,
				barH = mathMax(plotY, yBottom) - barY;

			// Handle options.minPointLength
			if (mathAbs(barH) < minPointLength) {
				if (minPointLength) {
					barH = minPointLength;
					up = (!yAxis.reversed && !point.negative) || (yAxis.reversed && point.negative);
					barY = mathAbs(barY - translatedThreshold) > minPointLength ? // stacked
							yBottom - minPointLength : // keep position
							translatedThreshold - (up ? minPointLength : 0); // #1485, #4051
				}
			}

			// Cache for access in polar
			point.barX = barX;
			point.pointWidth = pointWidth;

			// Fix the tooltip on center of grouped columns (#1216, #424, #3648)
			point.tooltipPos = chart.inverted ?
				[yAxis.len + yAxis.pos - chart.plotLeft - plotY, series.xAxis.len - barX - barW / 2, barH] :
				[barX + barW / 2, plotY + yAxis.pos - chart.plotTop, barH];

			// Register shape type and arguments to be used in drawPoints
			point.shapeType = 'rect';
			point.shapeArgs = series.crispCol(barX, barY, barW, barH);
		});

	},

	getSymbol: noop,

	/**
	 * Use a solid rectangle like the area series types
	 */
	drawLegendSymbol: LegendSymbolMixin.drawRectangle,


	/**
	 * Columns have no graph
	 */
	drawGraph: noop,

	/**
	 * Draw the columns. For bars, the series.group is rotated, so the same coordinates
	 * apply for columns and bars. This method is inherited by scatter series.
	 *
	 */
	drawPoints: function () {
		var series = this,
			chart = this.chart,
			options = series.options,
			renderer = chart.renderer,
			animationLimit = options.animationLimit || 250,
			shapeArgs,
			pointAttr;

		// draw the columns
		each(series.points, function (point) {
			var plotY = point.plotY,
				graphic = point.graphic,
				borderAttr;

			if (plotY !== UNDEFINED && !isNaN(plotY) && point.y !== null) {
				shapeArgs = point.shapeArgs;

				borderAttr = defined(series.borderWidth) ? {
					'stroke-width': series.borderWidth
				} : {};

				pointAttr = point.pointAttr[point.selected ? SELECT_STATE : NORMAL_STATE] || series.pointAttr[NORMAL_STATE];

				if (graphic) { // update
					stop(graphic);
					graphic.attr(borderAttr).attr(pointAttr)[chart.pointCount < animationLimit ? 'animate' : 'attr'](merge(shapeArgs)); // #4267

				} else {
					point.graphic = graphic = renderer[point.shapeType](shapeArgs)
						.attr(borderAttr)
						.attr(pointAttr)
						.add(point.group || series.group)
						.shadow(options.shadow, null, options.stacking && !options.borderRadius);
				}

			} else if (graphic) {
				point.graphic = graphic.destroy(); // #1269
			}
		});
	},

	/**
	 * Animate the column heights one by one from zero
	 * @param {Boolean} init Whether to initialize the animation or run it
	 */
	animate: function (init) {
		var series = this,
			yAxis = this.yAxis,
			options = series.options,
			inverted = this.chart.inverted,
			attr = {},
			translatedThreshold;

		if (hasSVG) { // VML is too slow anyway
			if (init) {
				attr.scaleY = 0.001;
				translatedThreshold = mathMin(yAxis.pos + yAxis.len, mathMax(yAxis.pos, yAxis.toPixels(options.threshold)));
				if (inverted) {
					attr.translateX = translatedThreshold - yAxis.len;
				} else {
					attr.translateY = translatedThreshold;
				}
				series.group.attr(attr);

			} else { // run the animation

				attr.scaleY = 1;
				attr[inverted ? 'translateX' : 'translateY'] = yAxis.pos;
				series.group.animate(attr, series.options.animation);

				// delete this function to allow it only once
				series.animate = null;
			}
		}
	},

	/**
	 * Remove this series from the chart
	 */
	remove: function () {
		var series = this,
			chart = series.chart;

		// column and bar series affects other series of the same type
		// as they are either stacked or grouped
		if (chart.hasRendered) {
			each(chart.series, function (otherSeries) {
				if (otherSeries.type === series.type) {
					otherSeries.isDirty = true;
				}
			});
		}

		Series.prototype.remove.apply(series, arguments);
	}
});
seriesTypes.column = ColumnSeries;
