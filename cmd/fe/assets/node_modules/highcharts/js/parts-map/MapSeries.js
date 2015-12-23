
// The vector-effect attribute is not supported in IE <= 11 (at least), so we need
// diffent logic (#3218)
var supportsVectorEffect = document.documentElement.style.vectorEffect !== undefined;

/**
 * Extend the default options with map options
 */
defaultPlotOptions.map = merge(defaultPlotOptions.scatter, {
	allAreas: true,

	animation: false, // makes the complex shapes slow
	nullColor: '#F8F8F8',
	borderColor: 'silver',
	borderWidth: 1,
	marker: null,
	stickyTracking: false,
	dataLabels: {
		formatter: function () { // #2945
			return this.point.value;
		},
		inside: true, // for the color
		verticalAlign: 'middle',
		crop: false,
		overflow: false,
		padding: 0
	},
	turboThreshold: 0,
	tooltip: {
		followPointer: true,
		pointFormat: '{point.name}: {point.value}<br/>'
	},
	states: {
		normal: {
			animation: true
		},
		hover: {
			brightness: 0.2,
			halo: null
		}
	}
});

/**
 * The MapAreaPoint object
 */
var MapAreaPoint = extendClass(Point, extend({
	/**
	 * Extend the Point object to split paths
	 */
	applyOptions: function (options, x) {

		var point = Point.prototype.applyOptions.call(this, options, x),
			series = this.series,
			joinBy = series.joinBy,
			mapPoint;

		if (series.mapData) {
			mapPoint = point[joinBy[1]] !== undefined && series.mapMap[point[joinBy[1]]];
			if (mapPoint) {
				// This applies only to bubbles
				if (series.xyFromShape) {
					point.x = mapPoint._midX;
					point.y = mapPoint._midY;
				}
				extend(point, mapPoint); // copy over properties
			} else {
				point.value = point.value || null;
			}
		}

		return point;
	},

	/**
	 * Stop the fade-out
	 */
	onMouseOver: function (e) {
		clearTimeout(this.colorInterval);
		if (this.value !== null) {
			Point.prototype.onMouseOver.call(this, e);
		} else { //#3401 Tooltip doesn't hide when hovering over null points
			this.series.onMouseOut(e);
		}
	},
	/**
	 * Custom animation for tweening out the colors. Animation reduces blinking when hovering
	 * over islands and coast lines. We run a custom implementation of animation becuase we
	 * need to be able to run this independently from other animations like zoom redraw. Also,
	 * adding color animation to the adapters would introduce almost the same amount of code.
	 */
	onMouseOut: function () {
		var point = this,
			start = +new Date(),
			normalColor = Color(point.color),
			hoverColor = Color(point.pointAttr.hover.fill),
			animation = point.series.options.states.normal.animation,
			duration = animation && (animation.duration || 500),
			fill;

		if (duration && normalColor.rgba.length === 4 && hoverColor.rgba.length === 4 && point.state !== 'select') {
			fill = point.pointAttr[''].fill;
			delete point.pointAttr[''].fill; // avoid resetting it in Point.setState

			clearTimeout(point.colorInterval);
			point.colorInterval = setInterval(function () {
				var pos = (new Date() - start) / duration,
					graphic = point.graphic;
				if (pos > 1) {
					pos = 1;
				}
				if (graphic) {
					graphic.attr('fill', ColorAxis.prototype.tweenColors.call(0, hoverColor, normalColor, pos));
				}
				if (pos >= 1) {
					clearTimeout(point.colorInterval);
				}
			}, 13);
		}
		Point.prototype.onMouseOut.call(point);

		if (fill) {
			point.pointAttr[''].fill = fill;
		}
	},

	/**
	 * Zoom the chart to view a specific area point
	 */
	zoomTo: function () {
		var point = this,
			series = point.series;

		series.xAxis.setExtremes(
			point._minX,
			point._maxX,
			false
		);
		series.yAxis.setExtremes(
			point._minY,
			point._maxY,
			false
		);
		series.chart.redraw();
	}
}, colorPointMixin)
);

/**
 * Add the series type
 */
seriesTypes.map = extendClass(seriesTypes.scatter, merge(colorSeriesMixin, {
	type: 'map',
	pointClass: MapAreaPoint,
	supportsDrilldown: true,
	getExtremesFromAll: true,
	useMapGeometry: true, // get axis extremes from paths, not values
	forceDL: true,
	searchPoint: noop,
	directTouch: true, // When tooltip is not shared, this series (and derivatives) requires direct touch/hover. KD-tree does not apply.
	preserveAspectRatio: true, // X axis and Y axis must have same translation slope
	/**
	 * Get the bounding box of all paths in the map combined.
	 */
	getBox: function (paths) {
		var MAX_VALUE = Number.MAX_VALUE,
			maxX = -MAX_VALUE,
			minX =  MAX_VALUE,
			maxY = -MAX_VALUE,
			minY =  MAX_VALUE,
			minRange = MAX_VALUE,
			xAxis = this.xAxis,
			yAxis = this.yAxis,
			hasBox;

		// Find the bounding box
		each(paths || [], function (point) {

			if (point.path) {
				if (typeof point.path === 'string') {
					point.path = Highcharts.splitPath(point.path);
				}

				var path = point.path || [],
					i = path.length,
					even = false, // while loop reads from the end
					pointMaxX = -MAX_VALUE,
					pointMinX =  MAX_VALUE,
					pointMaxY = -MAX_VALUE,
					pointMinY =  MAX_VALUE,
					properties = point.properties;

				// The first time a map point is used, analyze its box
				if (!point._foundBox) {
					while (i--) {
						if (typeof path[i] === 'number' && !isNaN(path[i])) {
							if (even) { // even = x
								pointMaxX = Math.max(pointMaxX, path[i]);
								pointMinX = Math.min(pointMinX, path[i]);
							} else { // odd = Y
								pointMaxY = Math.max(pointMaxY, path[i]);
								pointMinY = Math.min(pointMinY, path[i]);
							}
							even = !even;
						}
					}
					// Cache point bounding box for use to position data labels, bubbles etc
					point._midX = pointMinX + (pointMaxX - pointMinX) *
						(point.middleX || (properties && properties['hc-middle-x']) || 0.5); // pick is slower and very marginally needed
					point._midY = pointMinY + (pointMaxY - pointMinY) *
						(point.middleY || (properties && properties['hc-middle-y']) || 0.5);
					point._maxX = pointMaxX;
					point._minX = pointMinX;
					point._maxY = pointMaxY;
					point._minY = pointMinY;
					point.labelrank = pick(point.labelrank, (pointMaxX - pointMinX) * (pointMaxY - pointMinY));
					point._foundBox = true;
				}

				maxX = Math.max(maxX, point._maxX);
				minX = Math.min(minX, point._minX);
				maxY = Math.max(maxY, point._maxY);
				minY = Math.min(minY, point._minY);
				minRange = Math.min(point._maxX - point._minX, point._maxY - point._minY, minRange);
				hasBox = true;
			}
		});

		// Set the box for the whole series
		if (hasBox) {
			this.minY = Math.min(minY, pick(this.minY, MAX_VALUE));
			this.maxY = Math.max(maxY, pick(this.maxY, -MAX_VALUE));
			this.minX = Math.min(minX, pick(this.minX, MAX_VALUE));
			this.maxX = Math.max(maxX, pick(this.maxX, -MAX_VALUE));

			// If no minRange option is set, set the default minimum zooming range to 5 times the
			// size of the smallest element
			if (xAxis && xAxis.options.minRange === undefined) {
				xAxis.minRange = Math.min(5 * minRange, (this.maxX - this.minX) / 5, xAxis.minRange || MAX_VALUE);
			}
			if (yAxis && yAxis.options.minRange === undefined) {
				yAxis.minRange = Math.min(5 * minRange, (this.maxY - this.minY) / 5, yAxis.minRange || MAX_VALUE);
			}
		}
	},

	getExtremes: function () {
		// Get the actual value extremes for colors
		Series.prototype.getExtremes.call(this, this.valueData);

		// Recalculate box on updated data
		if (this.chart.hasRendered && this.isDirtyData) {
			this.getBox(this.options.data);
		}

		this.valueMin = this.dataMin;
		this.valueMax = this.dataMax;

		// Extremes for the mock Y axis
		this.dataMin = this.minY;
		this.dataMax = this.maxY;
	},

	/**
	 * Translate the path so that it automatically fits into the plot area box
	 * @param {Object} path
	 */
	translatePath: function (path) {

		var series = this,
			even = false, // while loop reads from the end
			xAxis = series.xAxis,
			yAxis = series.yAxis,
			xMin = xAxis.min,
			xTransA = xAxis.transA,
			xMinPixelPadding = xAxis.minPixelPadding,
			yMin = yAxis.min,
			yTransA = yAxis.transA,
			yMinPixelPadding = yAxis.minPixelPadding,
			i,
			ret = []; // Preserve the original

		// Do the translation
		if (path) {
			i = path.length;
			while (i--) {
				if (typeof path[i] === 'number') {
					ret[i] = even ?
						(path[i] - xMin) * xTransA + xMinPixelPadding :
						(path[i] - yMin) * yTransA + yMinPixelPadding;
					even = !even;
				} else {
					ret[i] = path[i];
				}
			}
		}

		return ret;
	},

	/**
	 * Extend setData to join in mapData. If the allAreas option is true, all areas
	 * from the mapData are used, and those that don't correspond to a data value
	 * are given null values.
	 */
	setData: function (data, redraw) {
		var options = this.options,
			mapData = options.mapData,
			joinBy = options.joinBy,
			joinByNull = joinBy === null,
			dataUsed = [],
			mapPoint,
			transform,
			mapTransforms,
			props,
			i;

		if (joinByNull) {
			joinBy = '_i';
		}
		joinBy = this.joinBy = Highcharts.splat(joinBy);
		if (!joinBy[1]) {
			joinBy[1] = joinBy[0];
		}

		// Pick up numeric values, add index
		if (data) {
			each(data, function (val, i) {
				if (typeof val === 'number') {
					data[i] = {
						value: val
					};
				}
				if (joinByNull) {
					data[i]._i = i;
				}
			});
		}

		this.getBox(data);
		if (mapData) {
			if (mapData.type === 'FeatureCollection') {
				if (mapData['hc-transform']) {
					this.chart.mapTransforms = mapTransforms = mapData['hc-transform'];
					// Cache cos/sin of transform rotation angle
					for (transform in mapTransforms) {
						if (mapTransforms.hasOwnProperty(transform) && transform.rotation) {
							transform.cosAngle = Math.cos(transform.rotation);
							transform.sinAngle = Math.sin(transform.rotation);
						}
					}
				}
				mapData = Highcharts.geojson(mapData, this.type, this);
			}

			this.getBox(mapData);
			this.mapData = mapData;
			this.mapMap = {};

			for (i = 0; i < mapData.length; i++) {
				mapPoint = mapData[i];
				props = mapPoint.properties;

				mapPoint._i = i;
				// Copy the property over to root for faster access
				if (joinBy[0] && props && props[joinBy[0]]) {
					mapPoint[joinBy[0]] = props[joinBy[0]];
				}
				this.mapMap[mapPoint[joinBy[0]]] = mapPoint;
			}

			if (options.allAreas) {

				data = data || [];

				// Registered the point codes that actually hold data
				if (joinBy[1]) {
					each(data, function (point) {
						dataUsed.push(point[joinBy[1]]);
					});
				}

				// Add those map points that don't correspond to data, which will be drawn as null points
				dataUsed = '|' + dataUsed.join('|') + '|'; // String search is faster than array.indexOf

				each(mapData, function (mapPoint) {
					if (!joinBy[0] || dataUsed.indexOf('|' + mapPoint[joinBy[0]] + '|') === -1) {
						data.push(merge(mapPoint, { value: null }));
					}
				});
			}
		}
		Series.prototype.setData.call(this, data, redraw);
	},


	/**
	 * No graph for the map series
	 */
	drawGraph: noop,

	/**
	 * We need the points' bounding boxes in order to draw the data labels, so
	 * we skip it now and call it from drawPoints instead.
	 */
	drawDataLabels: noop,

	/**
	 * Allow a quick redraw by just translating the area group. Used for zooming and panning
	 * in capable browsers.
	 */
	doFullTranslate: function () {
		return this.isDirtyData || this.chart.isResizing || this.chart.renderer.isVML || !this.baseTrans;
	},

	/**
	 * Add the path option for data points. Find the max value for color calculation.
	 */
	translate: function () {
		var series = this,
			xAxis = series.xAxis,
			yAxis = series.yAxis,
			doFullTranslate = series.doFullTranslate();

		series.generatePoints();

		each(series.data, function (point) {

			// Record the middle point (loosely based on centroid), determined
			// by the middleX and middleY options.
			point.plotX = xAxis.toPixels(point._midX, true);
			point.plotY = yAxis.toPixels(point._midY, true);

			if (doFullTranslate) {

				point.shapeType = 'path';
				point.shapeArgs = {
					d: series.translatePath(point.path)
				};
				if (supportsVectorEffect) {
					point.shapeArgs['vector-effect'] = 'non-scaling-stroke';
				}
			}
		});

		series.translateColors();
	},

	/**
	 * Use the drawPoints method of column, that is able to handle simple shapeArgs.
	 * Extend it by assigning the tooltip position.
	 */
	drawPoints: function () {
		var series = this,
			xAxis = series.xAxis,
			yAxis = series.yAxis,
			group = series.group,
			chart = series.chart,
			renderer = chart.renderer,
			scaleX,
			scaleY,
			translateX,
			translateY,
			baseTrans = this.baseTrans;

		// Set a group that handles transform during zooming and panning in order to preserve clipping
		// on series.group
		if (!series.transformGroup) {
			series.transformGroup = renderer.g()
				.attr({
					scaleX: 1,
					scaleY: 1
				})
				.add(group);
			series.transformGroup.survive = true;
		}

		// Draw the shapes again
		if (series.doFullTranslate()) {

			// Individual point actions
			if (chart.hasRendered && series.pointAttrToOptions.fill === 'color') {
				each(series.points, function (point) {

					// Reset color on update/redraw
					if (point.shapeArgs) {
						point.shapeArgs.fill = point.pointAttr[pick(point.state, '')].fill; // #3529
					}
				});
			}

			// If vector-effect is not supported, we set the stroke-width on the group element
			// and let all point graphics inherit. That way we don't have to iterate over all
			// points to update the stroke-width on zooming.
			if (!supportsVectorEffect) {
				each(series.points, function (point) {
					var attr = point.pointAttr[''];
					if (attr['stroke-width'] === series.pointAttr['']['stroke-width']) {
						attr['stroke-width'] = 'inherit';
					}
				});
			}

			// Draw them in transformGroup
			series.group = series.transformGroup;
			seriesTypes.column.prototype.drawPoints.apply(series);
			series.group = group; // Reset

			// Add class names
			each(series.points, function (point) {
				if (point.graphic) {
					if (point.name) {
						point.graphic.addClass('highcharts-name-' + point.name.replace(' ', '-').toLowerCase());
					}
					if (point.properties && point.properties['hc-key']) {
						point.graphic.addClass('highcharts-key-' + point.properties['hc-key'].toLowerCase());
					}

					if (!supportsVectorEffect) {
						point.graphic['stroke-widthSetter'] = noop;
					}
				}
			});

			// Set the base for later scale-zooming. The originX and originY properties are the
			// axis values in the plot area's upper left corner.
			this.baseTrans = {
				originX: xAxis.min - xAxis.minPixelPadding / xAxis.transA,
				originY: yAxis.min - yAxis.minPixelPadding / yAxis.transA + (yAxis.reversed ? 0 : yAxis.len / yAxis.transA),
				transAX: xAxis.transA,
				transAY: yAxis.transA
			};

			// Reset transformation in case we're doing a full translate (#3789)
			this.transformGroup.animate({
				translateX: 0,
				translateY: 0,
				scaleX: 1,
				scaleY: 1
			});

		// Just update the scale and transform for better performance
		} else {
			scaleX = xAxis.transA / baseTrans.transAX;
			scaleY = yAxis.transA / baseTrans.transAY;
			translateX = xAxis.toPixels(baseTrans.originX, true);
			translateY = yAxis.toPixels(baseTrans.originY, true);

			// Handle rounding errors in normal view (#3789)
			if (scaleX > 0.99 && scaleX < 1.01 && scaleY > 0.99 && scaleY < 1.01) {
				scaleX = 1;
				scaleY = 1;
				translateX = Math.round(translateX);
				translateY = Math.round(translateY);
			}

			this.transformGroup.animate({
				translateX: translateX,
				translateY: translateY,
				scaleX: scaleX,
				scaleY: scaleY
			});

		}

		// Set the stroke-width directly on the group element so the children inherit it. We need to use
		// setAttribute directly, because the stroke-widthSetter method expects a stroke color also to be
		// set.
		if (!supportsVectorEffect) {
			series.group.element.setAttribute('stroke-width', series.options.borderWidth / (scaleX || 1));
		}

		this.drawMapDataLabels();


	},

	/**
	 * Draw the data labels. Special for maps is the time that the data labels are drawn (after points),
	 * and the clipping of the dataLabelsGroup.
	 */
	drawMapDataLabels: function () {

		Series.prototype.drawDataLabels.call(this);
		if (this.dataLabelsGroup) {
			this.dataLabelsGroup.clip(this.chart.clipRect);
		}
	},

	/**
	 * Override render to throw in an async call in IE8. Otherwise it chokes on the US counties demo.
	 */
	render: function () {
		var series = this,
			render = Series.prototype.render;

		// Give IE8 some time to breathe.
		if (series.chart.renderer.isVML && series.data.length > 3000) {
			setTimeout(function () {
				render.call(series);
			});
		} else {
			render.call(series);
		}
	},

	/**
	 * The initial animation for the map series. By default, animation is disabled.
	 * Animation of map shapes is not at all supported in VML browsers.
	 */
	animate: function (init) {
		var chart = this.chart,
			animation = this.options.animation,
			group = this.group,
			xAxis = this.xAxis,
			yAxis = this.yAxis,
			left = xAxis.pos,
			top = yAxis.pos;

		if (chart.renderer.isSVG) {

			if (animation === true) {
				animation = {
					duration: 1000
				};
			}

			// Initialize the animation
			if (init) {

				// Scale down the group and place it in the center
				group.attr({
					translateX: left + xAxis.len / 2,
					translateY: top + yAxis.len / 2,
					scaleX: 0.001, // #1499
					scaleY: 0.001
				});

			// Run the animation
			} else {
				group.animate({
					translateX: left,
					translateY: top,
					scaleX: 1,
					scaleY: 1
				}, animation);

				// Delete this function to allow it only once
				this.animate = null;
			}
		}
	},

	/**
	 * Animate in the new series from the clicked point in the old series.
	 * Depends on the drilldown.js module
	 */
	animateDrilldown: function (init) {
		var toBox = this.chart.plotBox,
			level = this.chart.drilldownLevels[this.chart.drilldownLevels.length - 1],
			fromBox = level.bBox,
			animationOptions = this.chart.options.drilldown.animation,
			scale;

		if (!init) {

			scale = Math.min(fromBox.width / toBox.width, fromBox.height / toBox.height);
			level.shapeArgs = {
				scaleX: scale,
				scaleY: scale,
				translateX: fromBox.x,
				translateY: fromBox.y
			};

			each(this.points, function (point) {
				if (point.graphic) {
					point.graphic
						.attr(level.shapeArgs)
						.animate({
							scaleX: 1,
							scaleY: 1,
							translateX: 0,
							translateY: 0
						}, animationOptions);
				}
			});

			this.animate = null;
		}

	},

	drawLegendSymbol: LegendSymbolMixin.drawRectangle,

	/**
	 * When drilling up, pull out the individual point graphics from the lower series
	 * and animate them into the origin point in the upper series.
	 */
	animateDrillupFrom: function (level) {
		seriesTypes.column.prototype.animateDrillupFrom.call(this, level);
	},


	/**
	 * When drilling up, keep the upper series invisible until the lower series has
	 * moved into place
	 */
	animateDrillupTo: function (init) {
		seriesTypes.column.prototype.animateDrillupTo.call(this, init);
	}
}));
