


// The mapline series type
defaultPlotOptions.mapline = merge(defaultPlotOptions.map, {
	lineWidth: 1,
	fillColor: 'none'
});
seriesTypes.mapline = extendClass(seriesTypes.map, {
	type: 'mapline',
	pointAttrToOptions: { // mapping between SVG attributes and the corresponding options
		stroke: 'color',
		'stroke-width': 'lineWidth',
		fill: 'fillColor',
		dashstyle: 'dashStyle'
	},
	drawLegendSymbol: seriesTypes.line.prototype.drawLegendSymbol
});
