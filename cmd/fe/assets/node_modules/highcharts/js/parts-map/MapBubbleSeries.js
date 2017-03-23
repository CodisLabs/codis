

// The mapbubble series type
if (seriesTypes.bubble) {

	defaultPlotOptions.mapbubble = merge(defaultPlotOptions.bubble, {
		animationLimit: 500,
		tooltip: {
			pointFormat: '{point.name}: {point.z}'
		}
	});
	seriesTypes.mapbubble = extendClass(seriesTypes.bubble, {
		pointClass: extendClass(Point, {
			applyOptions: function (options, x) {
				var point;
				if (options && options.lat !== undefined && options.lon !== undefined) {
					point = Point.prototype.applyOptions.call(this, options, x);
					point = extend(point, this.series.chart.fromLatLonToPoint(point));
				} else {
					point = MapAreaPoint.prototype.applyOptions.call(this, options, x);
				}
				return point;
			},
			ttBelow: false
		}),
		xyFromShape: true,
		type: 'mapbubble',
		pointArrayMap: ['z'], // If one single value is passed, it is interpreted as z
		/**
		 * Return the map area identified by the dataJoinBy option
		 */
		getMapData: seriesTypes.map.prototype.getMapData,
		getBox: seriesTypes.map.prototype.getBox,
		setData: seriesTypes.map.prototype.setData
	});
}
