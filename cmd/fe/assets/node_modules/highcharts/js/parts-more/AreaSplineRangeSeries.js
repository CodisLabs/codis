/**
 * The AreaSplineRangeSeries class
 */

defaultPlotOptions.areasplinerange = merge(defaultPlotOptions.arearange);

/**
 * AreaSplineRangeSeries object
 */
seriesTypes.areasplinerange = extendClass(seriesTypes.arearange, {
	type: 'areasplinerange',
	getPointSpline: seriesTypes.spline.prototype.getPointSpline
});

