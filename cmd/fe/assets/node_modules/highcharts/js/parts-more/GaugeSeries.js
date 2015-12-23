/*
 * The GaugeSeries class
 */



/**
 * Extend the default options
 */
defaultPlotOptions.gauge = merge(defaultPlotOptions.line, {
	dataLabels: {
		enabled: true,
		defer: false,
		y: 15,
		borderWidth: 1,
		borderColor: 'silver',
		borderRadius: 3,
		crop: false,
		verticalAlign: 'top',
		zIndex: 2
	},
	dial: {
		// radius: '80%',
		// backgroundColor: 'black',
		// borderColor: 'silver',
		// borderWidth: 0,
		// baseWidth: 3,
		// topWidth: 1,
		// baseLength: '70%' // of radius
		// rearLength: '10%'
	},
	pivot: {
		//radius: 5,
		//borderWidth: 0
		//borderColor: 'silver',
		//backgroundColor: 'black'
	},
	tooltip: {
		headerFormat: ''
	},
	showInLegend: false
});

/**
 * Extend the point object
 */
var GaugePoint = extendClass(Point, {
	/**
	 * Don't do any hover colors or anything
	 */
	setState: function (state) {
		this.state = state;
	}
});


/**
 * Add the series type
 */
var GaugeSeries = {
	type: 'gauge',
	pointClass: GaugePoint,

	// chart.angular will be set to true when a gauge series is present, and this will
	// be used on the axes
	angular: true,
	drawGraph: noop,
	fixedBox: true,
	forceDL: true,
	trackerGroups: ['group', 'dataLabelsGroup'],

	/**
	 * Calculate paths etc
	 */
	translate: function () {

		var series = this,
			yAxis = series.yAxis,
			options = series.options,
			center = yAxis.center;

		series.generatePoints();

		each(series.points, function (point) {

			var dialOptions = merge(options.dial, point.dial),
				radius = (pInt(pick(dialOptions.radius, 80)) * center[2]) / 200,
				baseLength = (pInt(pick(dialOptions.baseLength, 70)) * radius) / 100,
				rearLength = (pInt(pick(dialOptions.rearLength, 10)) * radius) / 100,
				baseWidth = dialOptions.baseWidth || 3,
				topWidth = dialOptions.topWidth || 1,
				overshoot = options.overshoot,
				rotation = yAxis.startAngleRad + yAxis.translate(point.y, null, null, null, true);

			// Handle the wrap and overshoot options
			if (overshoot && typeof overshoot === 'number') {
				overshoot = overshoot / 180 * Math.PI;
				rotation = Math.max(yAxis.startAngleRad - overshoot, Math.min(yAxis.endAngleRad + overshoot, rotation));

			} else if (options.wrap === false) {
				rotation = Math.max(yAxis.startAngleRad, Math.min(yAxis.endAngleRad, rotation));
			}

			rotation = rotation * 180 / Math.PI;

			point.shapeType = 'path';
			point.shapeArgs = {
				d: dialOptions.path || [
					'M',
					-rearLength, -baseWidth / 2,
					'L',
					baseLength, -baseWidth / 2,
					radius, -topWidth / 2,
					radius, topWidth / 2,
					baseLength, baseWidth / 2,
					-rearLength, baseWidth / 2,
					'z'
				],
				translateX: center[0],
				translateY: center[1],
				rotation: rotation
			};

			// Positions for data label
			point.plotX = center[0];
			point.plotY = center[1];
		});
	},

	/**
	 * Draw the points where each point is one needle
	 */
	drawPoints: function () {

		var series = this,
			center = series.yAxis.center,
			pivot = series.pivot,
			options = series.options,
			pivotOptions = options.pivot,
			renderer = series.chart.renderer;

		each(series.points, function (point) {

			var graphic = point.graphic,
				shapeArgs = point.shapeArgs,
				d = shapeArgs.d,
				dialOptions = merge(options.dial, point.dial); // #1233

			if (graphic) {
				graphic.animate(shapeArgs);
				shapeArgs.d = d; // animate alters it
			} else {
				point.graphic = renderer[point.shapeType](shapeArgs)
					.attr({
						stroke: dialOptions.borderColor || 'none',
						'stroke-width': dialOptions.borderWidth || 0,
						fill: dialOptions.backgroundColor || 'black',
						rotation: shapeArgs.rotation, // required by VML when animation is false
						zIndex: 1
					})
					.add(series.group);
			}
		});

		// Add or move the pivot
		if (pivot) {
			pivot.animate({ // #1235
				translateX: center[0],
				translateY: center[1]
			});
		} else {
			series.pivot = renderer.circle(0, 0, pick(pivotOptions.radius, 5))
				.attr({
					'stroke-width': pivotOptions.borderWidth || 0,
					stroke: pivotOptions.borderColor || 'silver',
					fill: pivotOptions.backgroundColor || 'black',
					zIndex: 2
				})
				.translate(center[0], center[1])
				.add(series.group);
		}
	},

	/**
	 * Animate the arrow up from startAngle
	 */
	animate: function (init) {
		var series = this;

		if (!init) {
			each(series.points, function (point) {
				var graphic = point.graphic;

				if (graphic) {
					// start value
					graphic.attr({
						rotation: series.yAxis.startAngleRad * 180 / Math.PI
					});

					// animate
					graphic.animate({
						rotation: point.shapeArgs.rotation
					}, series.options.animation);
				}
			});

			// delete this function to allow it only once
			series.animate = null;
		}
	},

	render: function () {
		this.group = this.plotGroup(
			'group',
			'series',
			this.visible ? 'visible' : 'hidden',
			this.options.zIndex,
			this.chart.seriesGroup
		);
		Series.prototype.render.call(this);
		this.group.clip(this.chart.clipRect);
	},

	/**
	 * Extend the basic setData method by running processData and generatePoints immediately,
	 * in order to access the points from the legend.
	 */
	setData: function (data, redraw) {
		Series.prototype.setData.call(this, data, false);
		this.processData();
		this.generatePoints();
		if (pick(redraw, true)) {
			this.chart.redraw();
		}
	},

	/**
	 * If the tracking module is loaded, add the point tracker
	 */
	drawTracker: TrackerMixin && TrackerMixin.drawTrackerPoint
};
seriesTypes.gauge = extendClass(seriesTypes.line, GaugeSeries);

