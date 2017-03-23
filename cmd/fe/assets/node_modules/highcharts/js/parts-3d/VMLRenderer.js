/**
 *	Extension to the VML Renderer
 */
if (Highcharts.VMLRenderer) {

	Highcharts.setOptions({ animate: false });

	Highcharts.VMLRenderer.prototype.cuboid = Highcharts.SVGRenderer.prototype.cuboid;
	Highcharts.VMLRenderer.prototype.cuboidPath = Highcharts.SVGRenderer.prototype.cuboidPath;

	Highcharts.VMLRenderer.prototype.toLinePath = Highcharts.SVGRenderer.prototype.toLinePath;

	Highcharts.VMLRenderer.prototype.createElement3D = Highcharts.SVGRenderer.prototype.createElement3D;

	Highcharts.VMLRenderer.prototype.arc3d = function (shapeArgs) {
		var result = Highcharts.SVGRenderer.prototype.arc3d.call(this, shapeArgs);
		result.css({ zIndex: result.zIndex });
		return result;
	};

	Highcharts.VMLRenderer.prototype.arc3dPath = Highcharts.SVGRenderer.prototype.arc3dPath;

	Highcharts.wrap(Highcharts.Axis.prototype, 'render', function (proceed) {
		proceed.apply(this, [].slice.call(arguments, 1));
		// VML doesn't support a negative z-index
		if (this.sideFrame) {
			this.sideFrame.css({ zIndex: 0 });
			this.sideFrame.front.attr({ fill: this.sideFrame.color });
		}
		if (this.bottomFrame) {
			this.bottomFrame.css({ zIndex: 1 });
			this.bottomFrame.front.attr({ fill: this.bottomFrame.color });
		}
		if (this.backFrame) {
			this.backFrame.css({ zIndex: 0 });
			this.backFrame.front.attr({ fill: this.backFrame.color });
		}
	});

}
