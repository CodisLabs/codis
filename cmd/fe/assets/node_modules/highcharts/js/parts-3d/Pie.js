/***
	EXTENSION FOR 3D PIES
***/

Highcharts.wrap(Highcharts.seriesTypes.pie.prototype, 'translate', function (proceed) {
	proceed.apply(this, [].slice.call(arguments, 1));

	// Do not do this if the chart is not 3D
	if (!this.chart.is3d()) {
		return;
	}

	var series = this,
		chart = series.chart,
		options = chart.options,
		seriesOptions = series.options,
		depth = seriesOptions.depth || 0,
		options3d = options.chart.options3d,
		alpha = options3d.alpha,
		beta = options3d.beta,
		z = seriesOptions.stacking ? (seriesOptions.stack || 0) * depth : series._i * depth;

	z += depth / 2;

	if (seriesOptions.grouping !== false) {
		z = 0;
	}

	each(series.data, function (point) {

		var shapeArgs = point.shapeArgs,
			angle;

		point.shapeType = 'arc3d';

		shapeArgs.z = z;
		shapeArgs.depth = depth * 0.75;
		shapeArgs.alpha = alpha;
		shapeArgs.beta = beta;
		shapeArgs.center = series.center;

		angle = (shapeArgs.end + shapeArgs.start) / 2;

		point.slicedTranslation = {
			translateX: round(cos(angle) * seriesOptions.slicedOffset * cos(alpha * deg2rad)),
			translateY: round(sin(angle) * seriesOptions.slicedOffset * cos(alpha * deg2rad))
		};
	});
});

Highcharts.wrap(Highcharts.seriesTypes.pie.prototype.pointClass.prototype, 'haloPath', function (proceed) {
	var args = arguments;
	return this.series.chart.is3d() ? [] : proceed.call(this, args[1]);
});

Highcharts.wrap(Highcharts.seriesTypes.pie.prototype, 'drawPoints', function (proceed) {

	var options = this.options,
		states = options.states;

	// Do not do this if the chart is not 3D
	if (this.chart.is3d()) {
		// Set the border color to the fill color to provide a smooth edge
		this.borderWidth = options.borderWidth = options.edgeWidth || 1;
		this.borderColor = options.edgeColor = Highcharts.pick(options.edgeColor, options.borderColor, undefined);

		states.hover.borderColor = Highcharts.pick(states.hover.edgeColor, this.borderColor);
		states.hover.borderWidth = Highcharts.pick(states.hover.edgeWidth, this.borderWidth);
		states.select.borderColor = Highcharts.pick(states.select.edgeColor, this.borderColor);
		states.select.borderWidth = Highcharts.pick(states.select.edgeWidth, this.borderWidth);

		each(this.data, function (point) {
			var pointAttr = point.pointAttr;
			pointAttr[''].stroke = point.series.borderColor || point.color;
			pointAttr['']['stroke-width'] = point.series.borderWidth;
			pointAttr.hover.stroke = states.hover.borderColor;
			pointAttr.hover['stroke-width'] = states.hover.borderWidth;
			pointAttr.select.stroke = states.select.borderColor;
			pointAttr.select['stroke-width'] = states.select.borderWidth;
		});
	}

	proceed.apply(this, [].slice.call(arguments, 1));

	if (this.chart.is3d()) {
		each(this.points, function (point) {
			var graphic = point.graphic;

			// #4584 Check if has graphic - null points don't have it
			if (graphic) {
				// Hide null or 0 points (#3006, 3650)
				graphic[point.y ? 'show' : 'hide']();
			}
		});		
	}
});

Highcharts.wrap(Highcharts.seriesTypes.pie.prototype, 'drawDataLabels', function (proceed) {
	if (this.chart.is3d()) {
		var series = this,
			chart = series.chart,
			options3d = chart.options.chart.options3d;
		each(series.data, function (point) {
			var shapeArgs = point.shapeArgs,
				r = shapeArgs.r,
				a1 = (shapeArgs.alpha || options3d.alpha) * deg2rad, //#3240 issue with datalabels for 0 and null values
				b1 = (shapeArgs.beta || options3d.beta) * deg2rad,
				a2 = (shapeArgs.start + shapeArgs.end) / 2,
				labelPos = point.labelPos,
				labelIndexes = [0, 2, 4], // [x1, y1, x2, y2, x3, y3]
				yOffset = (-r * (1 - cos(a1)) * sin(a2)), // + (sin(a2) > 0 ? sin(a1) * d : 0)
				xOffset = r * (cos(b1) - 1) * cos(a2);

			// Apply perspective on label positions
			each(labelIndexes, function (index) {
				labelPos[index] += xOffset;
				labelPos[index + 1] += yOffset;
			});
		});
	}

	proceed.apply(this, [].slice.call(arguments, 1));
});

Highcharts.wrap(Highcharts.seriesTypes.pie.prototype, 'addPoint', function (proceed) {
	proceed.apply(this, [].slice.call(arguments, 1));
	if (this.chart.is3d()) {
		// destroy (and rebuild) everything!!!
		this.update(this.userOptions, true); // #3845 pass the old options
	}
});

Highcharts.wrap(Highcharts.seriesTypes.pie.prototype, 'animate', function (proceed) {
	if (!this.chart.is3d()) {
		proceed.apply(this, [].slice.call(arguments, 1));
	} else {
		var args = arguments,
			init = args[1],
			animation = this.options.animation,
			attribs,
			center = this.center,
			group = this.group,
			markerGroup = this.markerGroup;

		if (Highcharts.svg) { // VML is too slow anyway

			if (animation === true) {
				animation = {};
			}
			// Initialize the animation
			if (init) {

				// Scale down the group and place it in the center
				group.oldtranslateX = group.translateX;
				group.oldtranslateY = group.translateY;
				attribs = {
					translateX: center[0],
					translateY: center[1],
					scaleX: 0.001, // #1499
					scaleY: 0.001
				};

				group.attr(attribs);
				if (markerGroup) {
					markerGroup.attrSetters = group.attrSetters;
					markerGroup.attr(attribs);
				}

			// Run the animation
			} else {
				attribs = {
					translateX: group.oldtranslateX,
					translateY: group.oldtranslateY,
					scaleX: 1,
					scaleY: 1
				};
				group.animate(attribs, animation);

				if (markerGroup) {
					markerGroup.animate(attribs, animation);
				}

				// Delete this function to allow it only once
				this.animate = null;
			}

		}
	}
});
