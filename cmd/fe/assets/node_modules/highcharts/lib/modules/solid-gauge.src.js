/**
 * @license  Highcharts JS v4.1.10 (2015-12-07)
 * Solid angular gauge module
 *
 * (c) 2010-2014 Torstein Honsi
 *
 * License: www.highcharts.com/license
 */

(function (factory) {
	if (typeof module === 'object' && module.exports) {
		module.exports = factory;
	} else {
		factory(Highcharts);
	}
}(function (H) {
	'use strict';

	var defaultPlotOptions = H.getOptions().plotOptions,
		pInt = H.pInt,
		pick = H.pick,
		each = H.each,
		colorAxisMethods,
		UNDEFINED;

	// The default options
	defaultPlotOptions.solidgauge = H.merge(defaultPlotOptions.gauge, {
		colorByPoint: true
	});


	// These methods are defined in the ColorAxis object, and copied here.
	// If we implement an AMD system we should make ColorAxis a dependency.
	colorAxisMethods = {


		initDataClasses: function (userOptions) {
			var axis = this,
				chart = this.chart,
				dataClasses,
				colorCounter = 0,
				options = this.options;
			this.dataClasses = dataClasses = [];

			each(userOptions.dataClasses, function (dataClass, i) {
				var colors;

				dataClass = H.merge(dataClass);
				dataClasses.push(dataClass);
				if (!dataClass.color) {
					if (options.dataClassColor === 'category') {
						colors = chart.options.colors;
						dataClass.color = colors[colorCounter++];
						// loop back to zero
						if (colorCounter === colors.length) {
							colorCounter = 0;
						}
					} else {
						dataClass.color = axis.tweenColors(H.Color(options.minColor), H.Color(options.maxColor), i / (userOptions.dataClasses.length - 1));
					}
				}
			});
		},

		initStops: function (userOptions) {
			this.stops = userOptions.stops || [
				[0, this.options.minColor],
				[1, this.options.maxColor]
			];
			each(this.stops, function (stop) {
				stop.color = H.Color(stop[1]);
			});
		},
		/** 
		 * Translate from a value to a color
		 */
		toColor: function (value, point) {
			var pos,
				stops = this.stops,
				from,
				to,
				color,
				dataClasses = this.dataClasses,
				dataClass,
				i;

			if (dataClasses) {
				i = dataClasses.length;
				while (i--) {
					dataClass = dataClasses[i];
					from = dataClass.from;
					to = dataClass.to;
					if ((from === UNDEFINED || value >= from) && (to === UNDEFINED || value <= to)) {
						color = dataClass.color;
						if (point) {
							point.dataClass = i;
						}
						break;
					}   
				}

			} else {

				if (this.isLog) {
					value = this.val2lin(value);
				}
				pos = 1 - ((this.max - value) / (this.max - this.min));
				i = stops.length;
				while (i--) {
					if (pos > stops[i][0]) {
						break;
					}
				}
				from = stops[i] || stops[i + 1];
				to = stops[i + 1] || from;

				// The position within the gradient
				pos = 1 - (to[0] - pos) / ((to[0] - from[0]) || 1);
				
				color = this.tweenColors(
					from.color, 
					to.color,
					pos
				);
			}
			return color;
		},
		/*
		 * Return an intermediate color between two colors, according to pos where 0
		 * is the from color and 1 is the to color.
		 */
		tweenColors: function (from, to, pos) {
			// Check for has alpha, because rgba colors perform worse due to lack of
			// support in WebKit.
			var hasAlpha,
				ret;

			// Unsupported color, return to-color (#3920)
			if (!to.rgba.length || !from.rgba.length) {
				ret = to.input || 'none';

			// Interpolate
			} else {
				from = from.rgba;
				to = to.rgba;
				hasAlpha = (to[3] !== 1 || from[3] !== 1);
				ret = (hasAlpha ? 'rgba(' : 'rgb(') + 
					Math.round(to[0] + (from[0] - to[0]) * (1 - pos)) + ',' + 
					Math.round(to[1] + (from[1] - to[1]) * (1 - pos)) + ',' + 
					Math.round(to[2] + (from[2] - to[2]) * (1 - pos)) + 
					(hasAlpha ? (',' + (to[3] + (from[3] - to[3]) * (1 - pos))) : '') + ')';
			}
			return ret;
		}
	};

	/**
	 * Handle animation of the color attributes directly
	 */
	each(['fill', 'stroke'], function (prop) {
		H.addAnimSetter(prop, function (fx) {
			fx.elem.attr(prop, colorAxisMethods.tweenColors(H.Color(fx.start), H.Color(fx.end), fx.pos));
		});
	});

	// The series prototype
	H.seriesTypes.solidgauge = H.extendClass(H.seriesTypes.gauge, {
		type: 'solidgauge',
		pointAttrToOptions: {}, // #4301, don't inherit line marker's attribs
		bindAxes: function () {
			var axis;
			H.seriesTypes.gauge.prototype.bindAxes.call(this);

			axis = this.yAxis;
			H.extend(axis, colorAxisMethods);

			// Prepare data classes
			if (axis.options.dataClasses) {
				axis.initDataClasses(axis.options);
			}
			axis.initStops(axis.options);
		},

		/**
		 * Draw the points where each point is one needle
		 */
		drawPoints: function () {
			var series = this,
				yAxis = series.yAxis,
				center = yAxis.center,
				options = series.options,
				renderer = series.chart.renderer,
				overshoot = options.overshoot,
				overshootVal = overshoot && typeof overshoot === 'number' ? overshoot / 180 * Math.PI : 0;

			H.each(series.points, function (point) {
				var graphic = point.graphic,
					rotation = yAxis.startAngleRad + yAxis.translate(point.y, null, null, null, true),
					radius = (pInt(pick(point.options.radius, options.radius, 100)) * center[2]) / 200,
					innerRadius = (pInt(pick(point.options.innerRadius, options.innerRadius, 60)) * center[2]) / 200,
					shapeArgs,
					d,
					toColor = yAxis.toColor(point.y, point),
					axisMinAngle = Math.min(yAxis.startAngleRad, yAxis.endAngleRad),
					axisMaxAngle = Math.max(yAxis.startAngleRad, yAxis.endAngleRad),
					minAngle,
					maxAngle;

				if (toColor === 'none') { // #3708
					toColor = point.color || series.color || 'none';
				}
				if (toColor !== 'none') {
					point.color = toColor;
				}

				// Handle overshoot and clipping to axis max/min
				rotation = Math.max(axisMinAngle - overshootVal, Math.min(axisMaxAngle + overshootVal, rotation));

				// Handle the wrap option
				if (options.wrap === false) {
					rotation = Math.max(axisMinAngle, Math.min(axisMaxAngle, rotation));
				}

				minAngle = Math.min(rotation, yAxis.startAngleRad);
				maxAngle = Math.max(rotation, yAxis.startAngleRad);

				if (maxAngle - minAngle > 2 * Math.PI) {
					maxAngle = minAngle + 2 * Math.PI;
				}

				point.shapeArgs = shapeArgs = {
					x: center[0],
					y: center[1],
					r: radius,
					innerR: innerRadius,
					start: minAngle,
					end: maxAngle,
					fill: toColor
				};
				point.startR = radius; // For PieSeries.animate

				if (graphic) {
					d = shapeArgs.d;
					graphic.animate(shapeArgs);
					if (d) {
						shapeArgs.d = d; // animate alters it
					}
				} else {					
					point.graphic = renderer.arc(shapeArgs)
						.attr({
							stroke: options.borderColor || 'none',
							'stroke-width': options.borderWidth || 0,
							fill: toColor,
							'sweep-flag': 0
						})
						.add(series.group);
				}
			});
		},

		/**
		 * Extend the pie slice animation by animating from start angle and up
		 */
		animate: function (init) {

			if (!init) {
				this.startAngleRad = this.yAxis.startAngleRad;
				H.seriesTypes.pie.prototype.animate.call(this, init);
			}
		}
	});

}));
