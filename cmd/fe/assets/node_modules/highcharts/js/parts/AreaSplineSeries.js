/**
 * Set the default options for areaspline
 */
defaultPlotOptions.areaspline = merge(defaultPlotOptions.area);

/**
 * AreaSplineSeries object
 */
var areaProto = AreaSeries.prototype,
	AreaSplineSeries = extendClass(SplineSeries, {
		type: 'areaspline',
		closedStacks: true, // instead of following the previous graph back, follow the threshold back

		// Mix in methods from the area series
		getSegmentPath: areaProto.getSegmentPath,
		closeSegment: areaProto.closeSegment,
		drawGraph: areaProto.drawGraph,
		drawLegendSymbol: LegendSymbolMixin.drawRectangle
	});

seriesTypes.areaspline = AreaSplineSeries;

