/**
 * Set the default options for polygon
 */
defaultPlotOptions.polygon = merge(defaultPlotOptions.scatter, {
	marker: {
		enabled: false
	}
});

/**
 * The polygon series class
 */
seriesTypes.polygon = extendClass(seriesTypes.scatter, {
	type: 'polygon',
	fillGraph: true,
	// Close all segments
	getSegmentPath: function (segment) {
		return Series.prototype.getSegmentPath.call(this, segment).concat('z');
	},
	drawGraph: Series.prototype.drawGraph,
	drawLegendSymbol: Highcharts.LegendSymbolMixin.drawRectangle
});
