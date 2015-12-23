/**
 * Set the default options for area
 */
defaultPlotOptions.area = merge(defaultSeriesOptions, {
	softThreshold: false,
	threshold: 0
	// trackByArea: false,
	// lineColor: null, // overrides color, but lets fillColor be unaltered
	// fillOpacity: 0.75,
	// fillColor: null
});

/**
 * AreaSeries object
 */
var AreaSeries = extendClass(Series, {
	type: 'area',
	singleStacks: false,
	/**
	 * For stacks, don't split segments on null values. Instead, draw null values with
	 * no marker. Also insert dummy points for any X position that exists in other series
	 * in the stack.
	 */
	getSegments: function () {
		var series = this,
			segments = [],
			segment = [],
			keys = [],
			xAxis = this.xAxis,
			yAxis = this.yAxis,
			stack = yAxis.stacks[this.stackKey],
			pointMap = {},
			plotX,
			plotY,
			points = this.points,
			connectNulls = this.options.connectNulls,
			stackIndicator,
			i,
			x;

		if (this.options.stacking && !this.cropped) { // cropped causes artefacts in Stock, and perf issue
			// Create a map where we can quickly look up the points by their X value.
			for (i = 0; i < points.length; i++) {
				pointMap[points[i].x] = points[i];
			}

			// Sort the keys (#1651)
			for (x in stack) {
				if (stack[x].total !== null) { // nulled after switching between grouping and not (#1651, #2336)
					keys.push(+x);
				}
			}
			keys.sort(function (a, b) {
				return a - b;
			});

			each(keys, function (x) {
				var threshold = null,
					stackPoint,
					skip = connectNulls && (!pointMap[x] || pointMap[x].y === null); // #1836

				if (!skip) {

					// The point exists, push it to the segment
					if (pointMap[x]) {
						segment.push(pointMap[x]);

					// There is no point for this X value in this series, so we
					// insert a dummy point in order for the areas to be drawn
					// correctly.
					} else {

						// Loop down the stack to find the series below this one that has
						// a value (#1991)
						for (i = series.index; i <= yAxis.series.length; i++) {
							stackIndicator = series.getStackIndicator(null, x, i);
							stackPoint = stack[x].points[stackIndicator.key];
							if (stackPoint) {
								threshold = stackPoint[1];
								break;
							}
						}

						plotX = xAxis.translate(x);
						plotY = yAxis.getThreshold(threshold);
						segment.push({
							y: null,
							plotX: plotX,
							clientX: plotX,
							plotY: plotY,
							yBottom: plotY,
							onMouseOver: noop
						});
					}
				}
			});

			if (segment.length) {
				segments.push(segment);
			}

		} else {
			Series.prototype.getSegments.call(this);
			segments = this.segments;
		}

		this.segments = segments;
	},

	/**
	 * Extend the base Series getSegmentPath method by adding the path for the area.
	 * This path is pushed to the series.areaPath property.
	 */
	getSegmentPath: function (segment) {

		var segmentPath = Series.prototype.getSegmentPath.call(this, segment), // call base method
			areaSegmentPath = [].concat(segmentPath), // work on a copy for the area path
			i,
			options = this.options,
			segLength = segmentPath.length,
			translatedThreshold = this.yAxis.getThreshold(options.threshold), // #2181
			yBottom;

		if (segLength === 3) { // for animation from 1 to two points
			areaSegmentPath.push(L, segmentPath[1], segmentPath[2]);
		}
		if (options.stacking && !this.closedStacks) {

			// Follow stack back. Later, implement areaspline. A general solution could be to
			// reverse the entire graphPath of the previous series, though may be hard with
			// splines and with series with different extremes
			for (i = segment.length - 1; i >= 0; i--) {

				yBottom = pick(segment[i].yBottom, translatedThreshold);

				// step line?
				if (i < segment.length - 1 && options.step) {
					areaSegmentPath.push(segment[i + 1].plotX, yBottom);
				}

				areaSegmentPath.push(segment[i].plotX, yBottom);
			}

		} else { // follow zero line back
			this.closeSegment(areaSegmentPath, segment, translatedThreshold);
		}
		this.areaPath = this.areaPath.concat(areaSegmentPath);
		return segmentPath;
	},

	/**
	 * Extendable method to close the segment path of an area. This is overridden in polar
	 * charts.
	 */
	closeSegment: function (path, segment, translatedThreshold) {
		path.push(
			L,
			segment[segment.length - 1].plotX,
			translatedThreshold,
			L,
			segment[0].plotX,
			translatedThreshold
		);
	},

	/**
	 * Draw the graph and the underlying area. This method calls the Series base
	 * function and adds the area. The areaPath is calculated in the getSegmentPath
	 * method called from Series.prototype.drawGraph.
	 */
	drawGraph: function () {

		// Define or reset areaPath
		this.areaPath = [];

		// Call the base method
		Series.prototype.drawGraph.apply(this);

		// Define local variables
		var series = this,
			areaPath = this.areaPath,
			options = this.options,
			zones = this.zones,
			props = [['area', this.color, options.fillColor]]; // area name, main color, fill color

		each(zones, function (threshold, i) {
			props.push(['zoneArea' + i, threshold.color || series.color, threshold.fillColor || options.fillColor]);
		});
		each(props, function (prop) {
			var areaKey = prop[0],
				area = series[areaKey];

			// Create or update the area
			if (area) { // update
				area.animate({ d: areaPath });

			} else { // create
				series[areaKey] = series.chart.renderer.path(areaPath)
					.attr({
						fill: pick(
							prop[2],
							Color(prop[1]).setOpacity(pick(options.fillOpacity, 0.75)).get()
						),
						zIndex: 0 // #1069
					}).add(series.group);
			}
		});
	},

	drawLegendSymbol: LegendSymbolMixin.drawRectangle
});

seriesTypes.area = AreaSeries;
