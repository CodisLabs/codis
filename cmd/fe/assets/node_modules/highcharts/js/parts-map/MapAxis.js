/**
 * Override to use the extreme coordinates from the SVG shape, not the
 * data values
 */
wrap(Axis.prototype, 'getSeriesExtremes', function (proceed) {
	var isXAxis = this.isXAxis,
		dataMin,
		dataMax,
		xData = [],
		useMapGeometry;

	// Remove the xData array and cache it locally so that the proceed method doesn't use it
	if (isXAxis) {
		each(this.series, function (series, i) {
			if (series.useMapGeometry) {
				xData[i] = series.xData;
				series.xData = [];
			}
		});
	}

	// Call base to reach normal cartesian series (like mappoint)
	proceed.call(this);

	// Run extremes logic for map and mapline
	if (isXAxis) {
		dataMin = pick(this.dataMin, Number.MAX_VALUE);
		dataMax = pick(this.dataMax, -Number.MAX_VALUE);
		each(this.series, function (series, i) {
			if (series.useMapGeometry) {
				dataMin = Math.min(dataMin, pick(series.minX, dataMin));
				dataMax = Math.max(dataMax, pick(series.maxX, dataMin));
				series.xData = xData[i]; // Reset xData array
				useMapGeometry = true;
			}
		});
		if (useMapGeometry) {
			this.dataMin = dataMin;
			this.dataMax = dataMax;
		}
	}
});

/**
 * Override axis translation to make sure the aspect ratio is always kept
 */
wrap(Axis.prototype, 'setAxisTranslation', function (proceed) {
	var chart = this.chart,
		mapRatio,
		plotRatio = chart.plotWidth / chart.plotHeight,
		adjustedAxisLength,
		xAxis = chart.xAxis[0],
		padAxis,
		fixTo,
		fixDiff,
		preserveAspectRatio;


	// Run the parent method
	proceed.call(this);

	// Check for map-like series
	if (this.coll === 'yAxis' && xAxis.transA !== UNDEFINED) {
		each(this.series, function (series) {
			if (series.preserveAspectRatio) {
				preserveAspectRatio = true;
			}
		});
	}

	// On Y axis, handle both
	if (preserveAspectRatio) {

		// Use the same translation for both axes
		this.transA = xAxis.transA = Math.min(this.transA, xAxis.transA);

		mapRatio = plotRatio / ((xAxis.max - xAxis.min) / (this.max - this.min));

		// What axis to pad to put the map in the middle
		padAxis = mapRatio < 1 ? this : xAxis;

		// Pad it
		adjustedAxisLength = (padAxis.max - padAxis.min) * padAxis.transA;
		padAxis.pixelPadding = padAxis.len - adjustedAxisLength;
		padAxis.minPixelPadding = padAxis.pixelPadding / 2;

		fixTo = padAxis.fixTo;
		if (fixTo) {
			fixDiff = fixTo[1] - padAxis.toValue(fixTo[0], true);
			fixDiff *= padAxis.transA;
			if (Math.abs(fixDiff) > padAxis.minPixelPadding || (padAxis.min === padAxis.dataMin && padAxis.max === padAxis.dataMax)) { // zooming out again, keep within restricted area
				fixDiff = 0;
			}
			padAxis.minPixelPadding -= fixDiff;
		}
	}
});

/**
 * Override Axis.render in order to delete the fixTo prop
 */
wrap(Axis.prototype, 'render', function (proceed) {
	proceed.call(this);
	this.fixTo = null;
});