// Extend the Chart prototype for dynamic methods
extend(Chart.prototype, {

	/**
	 * Add a series dynamically after  time
	 *
	 * @param {Object} options The config options
	 * @param {Boolean} redraw Whether to redraw the chart after adding. Defaults to true.
	 * @param {Boolean|Object} animation Whether to apply animation, and optionally animation
	 *    configuration
	 *
	 * @return {Object} series The newly created series object
	 */
	addSeries: function (options, redraw, animation) {
		var series,
			chart = this;

		if (options) {
			redraw = pick(redraw, true); // defaults to true

			fireEvent(chart, 'addSeries', { options: options }, function () {
				series = chart.initSeries(options);

				chart.isDirtyLegend = true; // the series array is out of sync with the display
				chart.linkSeries();
				if (redraw) {
					chart.redraw(animation);
				}
			});
		}

		return series;
	},

	/**
     * Add an axis to the chart
     * @param {Object} options The axis option
     * @param {Boolean} isX Whether it is an X axis or a value axis
     */
	addAxis: function (options, isX, redraw, animation) {
		var key = isX ? 'xAxis' : 'yAxis',
			chartOptions = this.options;

		new Axis(this, merge(options, { // eslint-disable-line no-new
			index: this[key].length,
			isX: isX
		}));

		// Push the new axis options to the chart options
		chartOptions[key] = splat(chartOptions[key] || {});
		chartOptions[key].push(options);

		if (pick(redraw, true)) {
			this.redraw(animation);
		}
	},

	/**
	 * Dim the chart and show a loading text or symbol
	 * @param {String} str An optional text to show in the loading label instead of the default one
	 */
	showLoading: function (str) {
		var chart = this,
			options = chart.options,
			loadingDiv = chart.loadingDiv,
			loadingOptions = options.loading,
			setLoadingSize = function () {
				if (loadingDiv) {
					css(loadingDiv, {
						left: chart.plotLeft + PX,
						top: chart.plotTop + PX,
						width: chart.plotWidth + PX,
						height: chart.plotHeight + PX
					});
				}
			};

		// create the layer at the first call
		if (!loadingDiv) {
			chart.loadingDiv = loadingDiv = createElement(DIV, {
				className: PREFIX + 'loading'
			}, extend(loadingOptions.style, {
				zIndex: 10,
				display: NONE
			}), chart.container);

			chart.loadingSpan = createElement(
				'span',
				null,
				loadingOptions.labelStyle,
				loadingDiv
			);
			addEvent(chart, 'redraw', setLoadingSize); // #1080
		}

		// update text
		chart.loadingSpan.innerHTML = str || options.lang.loading;

		// show it
		if (!chart.loadingShown) {
			css(loadingDiv, {
				opacity: 0,
				display: ''
			});
			animate(loadingDiv, {
				opacity: loadingOptions.style.opacity
			}, {
				duration: loadingOptions.showDuration || 0
			});
			chart.loadingShown = true;
		}
		setLoadingSize();
	},

	/**
	 * Hide the loading layer
	 */
	hideLoading: function () {
		var options = this.options,
			loadingDiv = this.loadingDiv;

		if (loadingDiv) {
			animate(loadingDiv, {
				opacity: 0
			}, {
				duration: options.loading.hideDuration || 100,
				complete: function () {
					css(loadingDiv, { display: NONE });
				}
			});
		}
		this.loadingShown = false;
	}
});

// extend the Point prototype for dynamic methods
extend(Point.prototype, {
	/**
	 * Update the point with new options (typically x/y data) and optionally redraw the series.
	 *
	 * @param {Object} options Point options as defined in the series.data array
	 * @param {Boolean} redraw Whether to redraw the chart or wait for an explicit call
	 * @param {Boolean|Object} animation Whether to apply animation, and optionally animation
	 *    configuration
	 *
	 */
	update: function (options, redraw, animation, runEvent) {
		var point = this,
			series = point.series,
			graphic = point.graphic,
			i,
			chart = series.chart,
			seriesOptions = series.options,
			names = series.xAxis && series.xAxis.names;

		redraw = pick(redraw, true);

		function update() {

			point.applyOptions(options);

			// Update visuals
			if (point.y === null && graphic) { // #4146
				point.graphic = graphic.destroy();
			}
			if (isObject(options) && !isArray(options)) {
				// Defer the actual redraw until getAttribs has been called (#3260)
				point.redraw = function () {
					if (graphic && graphic.element) {
						if (options && options.marker && options.marker.symbol) {
							point.graphic = graphic.destroy();
						}
					}
					if (options && options.dataLabels && point.dataLabel) { // #2468
						point.dataLabel = point.dataLabel.destroy();
					}
					point.redraw = null;
				};
			}

			// record changes in the parallel arrays
			i = point.index;
			series.updateParallelArrays(point, i);
			if (names && point.name) {
				names[point.x] = point.name;
			}

			// Record the options to options.data. If there is an object from before,
			// use point options, otherwise use raw options. (#4701)
			seriesOptions.data[i] = isObject(seriesOptions.data[i]) ? point.options : options;

			// redraw
			series.isDirty = series.isDirtyData = true;
			if (!series.fixedBox && series.hasCartesianSeries) { // #1906, #2320
				chart.isDirtyBox = true;
			}

			if (seriesOptions.legendType === 'point') { // #1831, #1885
				chart.isDirtyLegend = true;
			}
			if (redraw) {
				chart.redraw(animation);
			}
		}

		// Fire the event with a default handler of doing the update
		if (runEvent === false) { // When called from setData
			update();
		} else {
			point.firePointEvent('update', { options: options }, update);
		}
	},

	/**
	 * Remove a point and optionally redraw the series and if necessary the axes
	 * @param {Boolean} redraw Whether to redraw the chart or wait for an explicit call
	 * @param {Boolean|Object} animation Whether to apply animation, and optionally animation
	 *    configuration
	 */
	remove: function (redraw, animation) {
		this.series.removePoint(inArray(this, this.series.data), redraw, animation);
	}
});

// Extend the series prototype for dynamic methods
extend(Series.prototype, {
	/**
	 * Add a point dynamically after chart load time
	 * @param {Object} options Point options as given in series.data
	 * @param {Boolean} redraw Whether to redraw the chart or wait for an explicit call
	 * @param {Boolean} shift If shift is true, a point is shifted off the start
	 *    of the series as one is appended to the end.
	 * @param {Boolean|Object} animation Whether to apply animation, and optionally animation
	 *    configuration
	 */
	addPoint: function (options, redraw, shift, animation) {
		var series = this,
			seriesOptions = series.options,
			data = series.data,
			graph = series.graph,
			area = series.area,
			chart = series.chart,
			names = series.xAxis && series.xAxis.names,
			currentShift = (graph && graph.shift) || 0,
			shiftShapes = ['graph', 'area'],
			dataOptions = seriesOptions.data,
			point,
			isInTheMiddle,
			xData = series.xData,
			i,
			x;

		setAnimation(animation, chart);

		// Make graph animate sideways
		if (shift) {
			i = series.zones.length;
			while (i--) {
				shiftShapes.push('zoneGraph' + i, 'zoneArea' + i);
			}
			each(shiftShapes, function (shape) {
				if (series[shape]) {
					series[shape].shift = currentShift + (seriesOptions.step ? 2 : 1);
				}
			});
		}
		if (area) {
			area.isArea = true; // needed in animation, both with and without shift
		}

		// Optional redraw, defaults to true
		redraw = pick(redraw, true);

		// Get options and push the point to xData, yData and series.options. In series.generatePoints
		// the Point instance will be created on demand and pushed to the series.data array.
		point = { series: series };
		series.pointClass.prototype.applyOptions.apply(point, [options]);
		x = point.x;

		// Get the insertion point
		i = xData.length;
		if (series.requireSorting && x < xData[i - 1]) {
			isInTheMiddle = true;
			while (i && xData[i - 1] > x) {
				i--;
			}
		}

		series.updateParallelArrays(point, 'splice', i, 0, 0); // insert undefined item
		series.updateParallelArrays(point, i); // update it

		if (names && point.name) {
			names[x] = point.name;
		}
		dataOptions.splice(i, 0, options);

		if (isInTheMiddle) {
			series.data.splice(i, 0, null);
			series.processData();
		}

		// Generate points to be added to the legend (#1329)
		if (seriesOptions.legendType === 'point') {
			series.generatePoints();
		}

		// Shift the first point off the parallel arrays
		if (shift) {
			if (data[0] && data[0].remove) {
				data[0].remove(false);
			} else {
				data.shift();
				series.updateParallelArrays(point, 'shift');

				dataOptions.shift();
			}
		}

		// redraw
		series.isDirty = true;
		series.isDirtyData = true;
		if (redraw) {
			series.getAttribs(); // #1937
			chart.redraw();
		}
	},

	/**
	 * Remove a point (rendered or not), by index
	 */
	removePoint: function (i, redraw, animation) {

		var series = this,
			data = series.data,
			point = data[i],
			points = series.points,
			chart = series.chart,
			remove = function () {

				if (data.length === points.length) {
					points.splice(i, 1);
				}
				data.splice(i, 1);
				series.options.data.splice(i, 1);
				series.updateParallelArrays(point || { series: series }, 'splice', i, 1);

				if (point) {
					point.destroy();
				}

				// redraw
				series.isDirty = true;
				series.isDirtyData = true;
				if (redraw) {
					chart.redraw();
				}
			};

		setAnimation(animation, chart);
		redraw = pick(redraw, true);

		// Fire the event with a default handler of removing the point
		if (point) {
			point.firePointEvent('remove', null, remove);
		} else {
			remove();
		}
	},

	/**
	 * Remove a series and optionally redraw the chart
	 *
	 * @param {Boolean} redraw Whether to redraw the chart or wait for an explicit call
	 * @param {Boolean|Object} animation Whether to apply animation, and optionally animation
	 *    configuration
	 */

	remove: function (redraw, animation) {
		var series = this,
			chart = series.chart;
		redraw = pick(redraw, true);

		if (!series.isRemoving) {  /* prevent triggering native event in jQuery
				(calling the remove function from the remove event) */
			series.isRemoving = true;

			// fire the event with a default handler of removing the point
			fireEvent(series, 'remove', null, function () {


				// destroy elements
				series.destroy();


				// redraw
				chart.isDirtyLegend = chart.isDirtyBox = true;
				chart.linkSeries();

				if (redraw) {
					chart.redraw(animation);
				}
			});

		}
		series.isRemoving = false;
	},

	/**
	 * Update the series with a new set of options
	 */
	update: function (newOptions, redraw) {
		var series = this,
			chart = this.chart,
			// must use user options when changing type because this.options is merged
			// in with type specific plotOptions
			oldOptions = this.userOptions,
			oldType = this.type,
			proto = seriesTypes[oldType].prototype,
			preserve = ['group', 'markerGroup', 'dataLabelsGroup'],
			n;

		// If we're changing type or zIndex, create new groups (#3380, #3404)
		if ((newOptions.type && newOptions.type !== oldType) || newOptions.zIndex !== undefined) {
			preserve.length = 0;
		}

		// Make sure groups are not destroyed (#3094)
		each(preserve, function (prop) {
			preserve[prop] = series[prop];
			delete series[prop];
		});

		// Do the merge, with some forced options
		newOptions = merge(oldOptions, {
			animation: false,
			index: this.index,
			pointStart: this.xData[0] // when updating after addPoint
		}, { data: this.options.data }, newOptions);

		// Destroy the series and delete all properties. Reinsert all methods
		// and properties from the new type prototype (#2270, #3719)
		this.remove(false);
		for (n in proto) {
			this[n] = UNDEFINED;
		}
		extend(this, seriesTypes[newOptions.type || oldType].prototype);

		// Re-register groups (#3094)
		each(preserve, function (prop) {
			series[prop] = preserve[prop];
		});

		this.init(chart, newOptions);
		chart.linkSeries(); // Links are lost in this.remove (#3028)
		if (pick(redraw, true)) {
			chart.redraw(false);
		}
	}
});

// Extend the Axis.prototype for dynamic methods
extend(Axis.prototype, {

	/**
	 * Update the axis with a new options structure
	 */
	update: function (newOptions, redraw) {
		var chart = this.chart;

		newOptions = chart.options[this.coll][this.options.index] = merge(this.userOptions, newOptions);

		this.destroy(true);
		this._addedPlotLB = this.chart._labelPanes = UNDEFINED; // #1611, #2887, #4314

		this.init(chart, extend(newOptions, { events: UNDEFINED }));

		chart.isDirtyBox = true;
		if (pick(redraw, true)) {
			chart.redraw();
		}
	},

	/**
     * Remove the axis from the chart
     */
	remove: function (redraw) {
		var chart = this.chart,
			key = this.coll, // xAxis or yAxis
			axisSeries = this.series,
			i = axisSeries.length;

		// Remove associated series (#2687)
		while (i--) {
			if (axisSeries[i]) {
				axisSeries[i].remove(false);
			}
		}

		// Remove the axis
		erase(chart.axes, this);
		erase(chart[key], this);
		chart.options[key].splice(this.options.index, 1);
		each(chart[key], function (axis, i) { // Re-index, #1706
			axis.options.index = i;
		});
		this.destroy();
		chart.isDirtyBox = true;

		if (pick(redraw, true)) {
			chart.redraw();
		}
	},

	/**
	 * Update the axis title by options
	 */
	setTitle: function (newTitleOptions, redraw) {
		this.update({ title: newTitleOptions }, redraw);
	},

	/**
	 * Set new axis categories and optionally redraw
	 * @param {Array} categories
	 * @param {Boolean} redraw
	 */
	setCategories: function (categories, redraw) {
		this.update({ categories: categories }, redraw);
	}

});

