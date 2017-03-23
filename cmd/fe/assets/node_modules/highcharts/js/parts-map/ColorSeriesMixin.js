/**
 * Mixin for maps and heatmaps
 */
var colorPointMixin = {
	/**
	 * Set the visibility of a single point
	 */
	setVisible: function (vis) {
		var point = this,
			method = vis ? 'show' : 'hide';

		// Show and hide associated elements
		each(['graphic', 'dataLabel'], function (key) {
			if (point[key]) {
				point[key][method]();
			}
		});
	}
};
var colorSeriesMixin = {

	pointAttrToOptions: { // mapping between SVG attributes and the corresponding options
		stroke: 'borderColor',
		'stroke-width': 'borderWidth',
		fill: 'color',
		dashstyle: 'dashStyle'
	},
	pointArrayMap: ['value'],
	axisTypes: ['xAxis', 'yAxis', 'colorAxis'],
	optionalAxis: 'colorAxis',
	trackerGroups: ['group', 'markerGroup', 'dataLabelsGroup'],
	getSymbol: noop,
	parallelArrays: ['x', 'y', 'value'],
	colorKey: 'value',

	/**
	 * In choropleth maps, the color is a result of the value, so this needs translation too
	 */
	translateColors: function () {
		var series = this,
			nullColor = this.options.nullColor,
			colorAxis = this.colorAxis,
			colorKey = this.colorKey;

		each(this.data, function (point) {
			var value = point[colorKey],
				color;

			color = point.options.color ||
				(value === null ? nullColor : (colorAxis && value !== undefined) ? colorAxis.toColor(value, point) : point.color || series.color);

			if (color) {
				point.color = color;
			}
		});
	}
};
