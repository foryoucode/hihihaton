def selected_needs(needs, selected):
    return sorted([r for r in needs if r['optional'] == 'no' or r['object_id'] in selected],
                  key=lambda r: (r['optional'] == 'yes', r['object_id'], r['equipment_type']))

def assignment(requirement, offer):
    return {'key': requirement['key'], 'object_id': requirement['object_id'],
            'equipment_type': requirement['equipment_type'], 'offer_id': offer['offer_id'],
            'equipment_id': offer['equipment_id'], 'model_name': offer['model']['model_name'],
            'supplier_id': offer['supplier_id'], 'supplier_name': offer['supplier']['supplier_name'],
            'unit_price_rub': offer['unit_price_rub'], 'ready_days': offer['ready_days']}
