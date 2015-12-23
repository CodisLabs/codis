/***
	EXTENSION FOR 3D SCATTER CHART
***/

Highcharts.wrap(Highcharts.seriesTypes.scatter.prototype, 'translate', function (proceed) {
//function translate3d(proceed) {
	proceed.apply(this, [].slice.call(arguments, 1));

	if (!this.chart.is3d()) {
		return;
	}

	var series = this,
		chart = series.chart,
		zAxis = Highcharts.pick(series.zAxis, chart.options.zAxis[0]),
		rawPoints = [],
		rawPoint,
		projectedPoints,
		projectedPoint,
		zValue,
		i;

	for (i = 0; i < series.data.length; i++) {
		rawPoint = series.data[i];
		zValue = zAxis.isLog && zAxis.val2lin ? zAxis.val2lin(rawPoint.z) : rawPoint.z; // #4562
		rawPoint.plotZ = zAxis.translate(zValue);

		rawPoint.isInside = rawPoint.isInside ? (zValue >= zAxis.min && zValue <= zAxis.max) : false;

		rawPoints.push({
			x: rawPoint.plotX,
			y: rawPoint.plotY,
			z: rawPoint.plotZ
		});
	}

	projectedPoints = perspective(rawPoints, chart, true);

	for (i = 0; i < series.data.length; i++) {
		rawPoint = series.data[i];
		projectedPoint = projectedPoints[i];

		rawPoint.plotXold = rawPoint.plotX;
		rawPoint.plotYold = rawPoint.plotY;

		rawPoint.plotX = projectedPoint.x;
		rawPoint.plotY = projectedPoint.y;
		rawPoint.plotZ = projectedPoint.z;


	}

});

Highcharts.wrap(Highcharts.seriesTypes.scatter.prototype, 'init', function (proceed, chart, options) {
	if (chart.is3d()) {
		// add a third coordinate
		this.axisTypes = ['xAxis', 'yAxis', 'zAxis'];
		this.pointArrayMap = ['x', 'y', 'z'];
		this.parallelArrays = ['x', 'y', 'z'];
	}

	var result = proceed.apply(this, [chart, options]);

	if (this.chart.is3d()) {
		// Set a new default tooltip formatter
		var default3dScatterTooltip = 'x: <b>{point.x}</b><br/>y: <b>{point.y}</b><br/>z: <b>{point.z}</b><br/>';
		if (this.userOptions.tooltip) {
			this.tooltipOptions.pointFormat = this.userOptions.tooltip.pointFormat || default3dScatterTooltip;
		} else {
			this.tooltipOptions.pointFormat = default3dScatterTooltip;
		}
	}
	return result;
});
