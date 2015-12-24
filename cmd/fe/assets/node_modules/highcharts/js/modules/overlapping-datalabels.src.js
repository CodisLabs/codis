/**
 * Highcharts module to hide overlapping data labels. This module is included in Highcharts.
 */
(function (H) {
	var Chart = H.Chart,
		each = H.each,
		pick = H.pick,
		addEvent = H.addEvent;

	// Collect potensial overlapping data labels. Stack labels probably don't need to be 
	// considered because they are usually accompanied by data labels that lie inside the columns.
	Chart.prototype.callbacks.push(function (chart) {
		function collectAndHide() {
			var labels = [];

			each(chart.series, function (series) {
				var dlOptions = series.options.dataLabels,
					collections = series.dataLabelCollections || ['dataLabel']; // Range series have two collections
				if ((dlOptions.enabled || series._hasPointLabels) && !dlOptions.allowOverlap && series.visible) { // #3866
					each(collections, function (coll) {
						each(series.points, function (point) {
							if (point[coll]) {
								point[coll].labelrank = pick(point.labelrank, point.shapeArgs && point.shapeArgs.height); // #4118
								labels.push(point[coll]);
							}
						});
					});
				}
			});
			chart.hideOverlappingLabels(labels);
		}

		// Do it now ...
		collectAndHide();

		// ... and after each chart redraw
		addEvent(chart, 'redraw', collectAndHide);

	});

	/**
	 * Hide overlapping labels. Labels are moved and faded in and out on zoom to provide a smooth 
	 * visual imression.
	 */		
	Chart.prototype.hideOverlappingLabels = function (labels) {

		var len = labels.length,
			label,
			i,
			j,
			label1,
			label2,
			isIntersecting,
			pos1,
			pos2,
			padding,
			intersectRect = function (x1, y1, w1, h1, x2, y2, w2, h2) {
				return !(
					x2 > x1 + w1 ||
					x2 + w2 < x1 ||
					y2 > y1 + h1 ||
					y2 + h2 < y1
				);
			};
	
		// Mark with initial opacity
		for (i = 0; i < len; i++) {
			label = labels[i];
			if (label) {
				label.oldOpacity = label.opacity;
				label.newOpacity = 1;
			}
		}

		// Prevent a situation in a gradually rising slope, that each label
		// will hide the previous one because the previous one always has
		// lower rank.
		labels.sort(function (a, b) {
			return (b.labelrank || 0) - (a.labelrank || 0);
		});

		// Detect overlapping labels
		for (i = 0; i < len; i++) {
			label1 = labels[i];

			for (j = i + 1; j < len; ++j) {
				label2 = labels[j];
				if (label1 && label2 && label1.placed && label2.placed && label1.newOpacity !== 0 && label2.newOpacity !== 0) {
					pos1 = label1.alignAttr;
					pos2 = label2.alignAttr;
					padding = 2 * (label1.box ? 0 : label1.padding); // Substract the padding if no background or border (#4333)
					isIntersecting = intersectRect(
						pos1.x,
						pos1.y,
						label1.width - padding,
						label1.height - padding,
						pos2.x,
						pos2.y,
						label2.width - padding,
						label2.height - padding
					);

					if (isIntersecting) {
						(label1.labelrank < label2.labelrank ? label1 : label2).newOpacity = 0;
					}
				}
			}
		}

		// Hide or show
		each(labels, function (label) {
			var complete,
				newOpacity;

			if (label) {
				newOpacity = label.newOpacity;

				if (label.oldOpacity !== newOpacity && label.placed) {

					// Make sure the label is completely hidden to avoid catching clicks (#4362)
					if (newOpacity) {
						label.show(true);
					} else {
						complete = function () {
							label.hide();
						};
					}

					// Animate or set the opacity					
					label.alignAttr.opacity = newOpacity;
					label[label.isOld ? 'animate' : 'attr'](label.alignAttr, null, complete);
					
				}
				label.isOld = true;
			}
		});
	};
}(Highcharts));
