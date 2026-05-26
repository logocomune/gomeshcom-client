class ViewStore {
	mobileDrawerOpen = $state(false);

	openDrawer() {
		this.mobileDrawerOpen = true;
	}

	closeDrawer() {
		this.mobileDrawerOpen = false;
	}
}

export const viewState = new ViewStore();
